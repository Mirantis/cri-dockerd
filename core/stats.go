/*
Copyright 2021 Mirantis

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type cstats struct {
	sync.Mutex
	ds          *dockerService
	containerID string
	stopCh      chan struct{}
	rwLayerSize uint64
	initialized bool
}

type containerStatsCache struct {
	sync.RWMutex
	stats map[string]*cstats
	clist chan []*runtimeapi.Container
}

func newCstats(cid string, ds *dockerService) *cstats {
	return &cstats{
		containerID: cid,
		ds:          ds,
		stopCh:      make(chan struct{}),
	}
}

func newContainerStatsCache() *containerStatsCache {
	return &containerStatsCache{
		stats: make(map[string]*cstats),
		clist: make(chan []*runtimeapi.Container, 1),
	}
}

const maxBackoffDuration = 20 * time.Minute
const minCollectInterval = time.Minute

func (cs *cstats) startCollect() {
	backoffDuration := minCollectInterval
	for {
		var sleepTime time.Duration
		// time consuming operation
		start := time.Now()
		containerJSON, err := cs.ds.client.InspectContainerWithSize(cs.containerID)
		logrus.Debugf("Get RW layer size for container ID '%s', time taken %v", cs.containerID, time.Since(start))
		if err != nil {
			logrus.Errorf("error getting RW layer size for container ID '%s': %v", cs.containerID, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				backoffDuration = backoffDuration * 2
				if backoffDuration >= maxBackoffDuration {
					backoffDuration = maxBackoffDuration
				}
			}
			logrus.Errorf("Set backoffDuration to : %v for container ID '%s'", backoffDuration, cs.containerID)
			sleepTime = backoffDuration
		} else {
			cs.Lock()
			cs.rwLayerSize = uint64(*containerJSON.SizeRw)
			cs.initialized = true
			cs.Unlock()
			backoffDuration = minCollectInterval
			logrus.Debugf("RW layer size for container ID '%s': %v", cs.containerID, cs.rwLayerSize)
			sleepTime = minCollectInterval
		}
		select {
		case <-cs.stopCh:
			return
		case <-time.After(sleepTime):
		}
	}
}

func (cs *cstats) stopCollect() {
	cs.stopCh <- struct{}{}
}

func (cs *cstats) isInitialized() bool {
	cs.Lock()
	defer cs.Unlock()
	return cs.initialized
}

func (cs *cstats) getContainerRWSize() uint64 {
	cs.Lock()
	defer cs.Unlock()
	return cs.rwLayerSize
}

func (c *containerStatsCache) getWriteableLayer() uint64 {
	c.RLock()
	defer c.RUnlock()

	var totalLayerSize uint64
	for _, stat := range c.stats {
		totalLayerSize += stat.rwLayerSize
	}
	return totalLayerSize
}

func (c *containerStatsCache) getStats(containerID string) *cstats {
	c.RLock()
	defer c.RUnlock()
	if _, exist := c.stats[containerID]; !exist {
		return nil
	}
	return c.stats[containerID]
}

func (ds *dockerService) startStatsCollection() {
	c := ds.containerStatsCache
	for clist := range c.clist {
		c.Lock()
		containerIDMap := make(map[string]struct{}, len(clist))
		for _, container := range clist {
			cid := container.Id
			containerIDMap[cid] = struct{}{}
			// add new container
			if _, exist := c.stats[cid]; !exist {
				cs := newCstats(cid, ds)
				c.stats[cid] = cs
				go cs.startCollect()
			}
		}
		// cleanup the containers that are not in latest container list
		for k, cs := range c.stats {
			if _, exist := containerIDMap[k]; !exist {
				delete(c.stats, k)
				go cs.stopCollect()
			}
		}
		c.Unlock()
	}
}

// ContainerStats returns stats for a container stats request based on container id.
func (ds *dockerService) ContainerStats(
	ctx context.Context,
	r *runtimeapi.ContainerStatsRequest,
) (*runtimeapi.ContainerStatsResponse, error) {
	filter := &runtimeapi.ContainerFilter{
		Id: r.ContainerId,
	}
	listResp, err := ds.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	if len(listResp.Containers) != 1 {
		return nil, fmt.Errorf("container with id %s not found", r.ContainerId)
	}
	stats, err := ds.getContainerStats(listResp.Containers[0])
	if err != nil {
		return nil, err
	}
	if stats == nil {
		return nil, fmt.Errorf("stats for container with id %s not available", r.ContainerId)
	}
	return &runtimeapi.ContainerStatsResponse{Stats: stats}, nil
}

// ListContainerStats returns stats for a list container stats request based on a filter.
func (ds *dockerService) ListContainerStats(
	ctx context.Context,
	r *runtimeapi.ListContainerStatsRequest,
) (*runtimeapi.ListContainerStatsResponse, error) {
	start := time.Now()
	containerStatsFilter := r.GetFilter()
	filter := &runtimeapi.ContainerFilter{}

	if containerStatsFilter != nil {
		filter.Id = containerStatsFilter.Id
		filter.PodSandboxId = containerStatsFilter.PodSandboxId
		filter.LabelSelector = containerStatsFilter.LabelSelector
	}

	res, err := ds.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: filter})
	if err != nil {
		logrus.Errorf("Error listing containers with filter: %+v", filter)
		logrus.Errorf("Error listing containers error: %s", err)
		return nil, err
	}
	containers := res.Containers
	ds.containerStatsCache.clist <- containers
	numContainers := len(containers)
	logrus.Debugf("Number of pod containers: %v", numContainers)
	if numContainers == 0 {
		return &runtimeapi.ListContainerStatsResponse{}, nil
	}

	var mu sync.Mutex
	results := make([]*runtimeapi.ContainerStats, 0, len(containers))

	g, ctx := errgroup.WithContext(ctx)
	// The `getContainerStats` may take some time. When there are many containers,
	// the whole `ListContainerStats` may have long delays if the number of workers is
	// small. So we want to set a bigger value for the number of workers to avoid
	// too long delays before the issue mentioned in https://github.com/moby/moby/pull/46448
	// is fixed.
	// Consider a common node with 8 CPU running dozens of pods, NumCPU() * 6 may be a moderate
	// number.
	numWorkers := runtime.NumCPU() * 6
	if numWorkers > numContainers {
		numWorkers = numContainers
	}
	g.SetLimit(numWorkers)

	// Collect container stats and send to result channel.
	// The concurrency is numWorkers
	for _, c := range containers {
		c := c
		g.Go(func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			stats, err := ds.getContainerStats(c)
			if err != nil {
				logrus.Errorf("error collecting stats for container '%s': %v", c.Metadata.Name, err)
				return nil
			}
			mu.Lock()
			if stats != nil {
				results = append(results, stats)
			}
			mu.Unlock()
			return nil
		})
	}

	// wait for workers to finish
	if err := g.Wait(); err != nil {
		logrus.Errorf("Error ListContainerStats. %v", err)
		return nil, err
	}

	logrus.Debugf("Number of stats:%v, Time taken: %v", len(results), time.Since(start))

	return &runtimeapi.ListContainerStatsResponse{Stats: results}, nil
}
