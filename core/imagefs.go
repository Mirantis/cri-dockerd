package core

import (
	"context"
	"time"

	"github.com/Mirantis/cri-dockerd/utils"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ImageFsStatsCache caches imagefs stats.
var ImageFsStatsCache utils.Cache

const imageFsStatsMinTTL = 30 * time.Second

// ImageFsInfo returns information of the filesystem that is used to store images.
func (ds *dockerService) ImageFsInfo(
	_ context.Context,
	_ *runtimeapi.ImageFsInfoRequest,
) (*runtimeapi.ImageFsInfoResponse, error) {

	res, err := ImageFsStatsCache.Memoize("imagefs", imageFsStatsMinTTL, func() (interface{}, error) {
		return ds.imageFsInfo()
	})
	if err != nil {
		return nil, err
	}
	stats := res.(*runtimeapi.ImageFsInfoResponse)
	return stats, nil

}
