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

package libdocker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	dockertypes "github.com/docker/docker/api/types"
	dockerbackend "github.com/docker/docker/api/types/backend"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimagetypes "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	dockersystem "github.com/docker/docker/api/types/system"
	dockerapi "github.com/docker/docker/client"
	dockermessage "github.com/docker/docker/pkg/jsonmessage"
	dockerstdcopy "github.com/docker/docker/pkg/stdcopy"
)

// kubeDockerClient is a wrapped layer of docker client for kubelet internal use. This layer is added to:
//  1. Redirect stream for exec and attach operations.
//  2. Wrap the context in this layer to make the DockerClientInterface cleaner.
type kubeDockerClient struct {
	// timeout is the timeout of short running docker operations.
	timeout time.Duration
	// If no pulling progress is made before imagePullProgressDeadline, the image pulling will be cancelled.
	// Docker reports image progress for every 512kB block, so normally there shouldn't be too long interval
	// between progress updates.
	imagePullProgressDeadline time.Duration
	client                    *dockerapi.Client
}

// Make sure that kubeDockerClient implemented the DockerClientInterface.
var _ DockerClientInterface = &kubeDockerClient{}

// There are 2 kinds of docker operations categorized by running time:
// * Long running operation: The long running operation could run for arbitrary long time, and the running time
// usually depends on some uncontrollable factors. These operations include: PullImage, Logs, StartExec, AttachToContainer.
// * Non-long running operation: Given the maximum load of the system, the non-long running operation should finish
// in expected and usually short time. These include all other operations.
// kubeDockerClient only applies timeout on non-long running operations.
const (
	// defaultTimeout is the default timeout of short running docker operations.
	// Value is slightly offset from 2 minutes to make timeouts due to this
	// constant recognizable.
	defaultTimeout = 2*time.Minute - 1*time.Second

	// defaultShmSize is the default ShmSize to use (in bytes) if not specified.
	defaultShmSize = int64(1024 * 1024 * 64)

	// defaultImagePullingProgressReportInterval is the default interval of image pulling progress reporting.
	defaultImagePullingProgressReportInterval = 10 * time.Second
)

// newKubeDockerClient creates an kubeDockerClient from an existing docker client. If requestTimeout is 0,
// defaultTimeout will be applied.
func newKubeDockerClient(
	dockerClient *dockerapi.Client,
	requestTimeout, imagePullProgressDeadline time.Duration,
) DockerClientInterface {
	if requestTimeout == 0 {
		requestTimeout = defaultTimeout
	}

	k := &kubeDockerClient{
		client:                    dockerClient,
		timeout:                   requestTimeout,
		imagePullProgressDeadline: imagePullProgressDeadline,
	}

	// Notice that this assumes that docker is running before kubelet is started.
	ctx, cancel := context.WithTimeout(context.Background(), k.timeout)
	defer cancel()
	dockerClient.NegotiateAPIVersion(ctx)

	return k
}

func (d *kubeDockerClient) ListContainers(
	options dockercontainer.ListOptions,
) ([]dockertypes.Container, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	containers, err := d.client.ContainerList(ctx, options)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return containers, nil
}

func (d *kubeDockerClient) InspectContainer(id string) (*dockertypes.ContainerJSON, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	containerJSON, err := d.client.ContainerInspect(ctx, id)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &containerJSON, nil
}

// InspectContainerWithSize is currently only used for Windows container stats
func (d *kubeDockerClient) InspectContainerWithSize(id string) (*dockertypes.ContainerJSON, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	// Inspects the container including the fields SizeRw and SizeRootFs.
	containerJSON, _, err := d.client.ContainerInspectWithRaw(ctx, id, true)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &containerJSON, nil
}

func (d *kubeDockerClient) CreateContainer(
	opts dockerbackend.ContainerCreateConfig,
) (*dockercontainer.CreateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	// we provide an explicit default shm size as to not depend on docker daemon.
	if opts.HostConfig != nil && opts.HostConfig.ShmSize <= 0 {
		opts.HostConfig.ShmSize = defaultShmSize
	}
	createResp, err := d.client.ContainerCreate(
		ctx,
		opts.Config,
		opts.HostConfig,
		opts.NetworkingConfig,
		nil,
		opts.Name,
	)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &createResp, nil
}

func (d *kubeDockerClient) StartContainer(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.client.ContainerStart(ctx, id, dockercontainer.StartOptions{})
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (d *kubeDockerClient) StopContainer(id string, timeout time.Duration) error {
	ctx, cancel := d.getCustomTimeoutContext(timeout)
	defer cancel()
	timeoutSeconds := int(timeout.Seconds())
	options := dockercontainer.StopOptions{
		Timeout: &timeoutSeconds,
	}

	err := d.client.ContainerStop(ctx, id, options)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (d *kubeDockerClient) RemoveContainer(
	id string,
	opts dockercontainer.RemoveOptions,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	err := d.client.ContainerRemove(ctx, id, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (d *kubeDockerClient) UpdateContainerResources(
	id string,
	updateConfig dockercontainer.UpdateConfig,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	_, err := d.client.ContainerUpdate(ctx, id, updateConfig)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func (d *kubeDockerClient) inspectImageRaw(ref string) (*dockertypes.ImageInspect, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, _, err := d.client.ImageInspectWithRaw(ctx, ref)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		if dockerapi.IsErrNotFound(err) {
			err = ImageNotFoundError{ID: ref}
		}
		return nil, err
	}

	return &resp, nil
}

func (d *kubeDockerClient) InspectImageByID(imageID string) (*dockertypes.ImageInspect, error) {
	resp, err := d.inspectImageRaw(imageID)
	if err != nil {
		return nil, err
	}

	if !matchImageIDOnly(*resp, imageID) {
		return nil, ImageNotFoundError{ID: imageID}
	}
	return resp, nil
}

func (d *kubeDockerClient) InspectImageByRef(imageRef string) (*dockertypes.ImageInspect, error) {
	resp, err := d.inspectImageRaw(imageRef)
	if err != nil {
		return nil, err
	}

	if !matchImageTagOrSHA(*resp, imageRef) {
		return nil, ImageNotFoundError{ID: imageRef}
	}
	return resp, nil
}

func (d *kubeDockerClient) ImageHistory(id string) ([]dockerimagetypes.HistoryResponseItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.ImageHistory(ctx, id)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return resp, err
}

func (d *kubeDockerClient) ListImages(
	opts dockerimagetypes.ListOptions,
) ([]dockerimagetypes.Summary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	images, err := d.client.ImageList(ctx, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return images, nil
}

func base64EncodeAuth(auth dockerregistry.AuthConfig) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(auth); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

// progress is a wrapper of dockermessage.JSONMessage with a lock protecting it.
type progress struct {
	sync.RWMutex
	// message stores the latest docker json message.
	message *dockermessage.JSONMessage
	// timestamp of the latest update.
	timestamp time.Time
}

func newProgress() *progress {
	return &progress{timestamp: time.Now()}
}

func (p *progress) set(msg *dockermessage.JSONMessage) {
	p.Lock()
	defer p.Unlock()
	p.message = msg
	p.timestamp = time.Now()
}

func (p *progress) get() (*dockermessage.JSONMessage, time.Time) {
	p.RLock()
	defer p.RUnlock()
	return p.message, p.timestamp
}

func formatProgress(msg *dockermessage.JSONMessage) string {
	if msg == nil {
		return "No progress"
	}
	// The following code is based on JSONMessage.Display
	var prefix string
	if msg.ID != "" {
		prefix = fmt.Sprintf("%s: ", msg.ID)
	}
	if msg.Progress == nil {
		return fmt.Sprintf("%s%s", prefix, msg.Status)
	}
	return fmt.Sprintf(
		"%s%s %s",
		prefix,
		msg.Status,
		msg.Progress.String(),
	)
}

// progressReporter keeps the newest image pulling progress and periodically report the newest progress.
type progressReporter struct {
	*progress
	image                     string
	cancel                    context.CancelFunc
	stopCh                    chan struct{}
	imagePullProgressDeadline time.Duration
}

// newProgressReporter creates a new progressReporter for specific image with specified reporting interval
func newProgressReporter(
	image string,
	cancel context.CancelFunc,
	imagePullProgressDeadline time.Duration,
) *progressReporter {
	return &progressReporter{
		progress:                  newProgress(),
		image:                     image,
		cancel:                    cancel,
		stopCh:                    make(chan struct{}),
		imagePullProgressDeadline: imagePullProgressDeadline,
	}
}

// start starts the progressReporter
func (p *progressReporter) start() {
	go func() {
		ticker := time.NewTicker(defaultImagePullingProgressReportInterval)
		defer ticker.Stop()
		downloaded := false
		for {
			select {
			case <-ticker.C:
				progress, timestamp := p.progress.get()
				if progress.Status == "Extracting" {
					downloaded = true
				}
				// If there is no progress for p.imagePullProgressDeadline in 'downloading' phase, cancel the operation.
				if !downloaded && time.Since(timestamp) > p.imagePullProgressDeadline {
					logrus.Errorf(
						"Cancel pulling image %s because it exceeded image pull deadline %s. Latest progress %s",
						p.image,
						p.imagePullProgressDeadline.String(),
						formatProgress(progress),
					)
					p.cancel()
					return
				}
				logrus.Infof("Pulling image %s: %s", p.image, formatProgress(progress))
			case <-p.stopCh:
				progress, _ := p.progress.get()
				logrus.Infof("Stop pulling image %s: %s", p.image, formatProgress(progress))
				return
			}
		}
	}()
}

// stop stops the progressReporter
func (p *progressReporter) stop() {
	close(p.stopCh)
}

func (d *kubeDockerClient) PullImage(
	image string,
	auth dockerregistry.AuthConfig,
	opts dockerimagetypes.PullOptions,
) error {
	// RegistryAuth is the base64 encoded credentials for the registry
	base64Auth, err := base64EncodeAuth(auth)
	if err != nil {
		return err
	}
	opts.RegistryAuth = base64Auth
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := d.client.ImagePull(ctx, image, opts)
	if err != nil {
		return err
	}
	defer resp.Close()
	reporter := newProgressReporter(image, cancel, d.imagePullProgressDeadline)
	reporter.start()
	defer reporter.stop()
	decoder := json.NewDecoder(resp)
	for {
		var msg dockermessage.JSONMessage
		err := decoder.Decode(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if msg.Error != nil {
			return msg.Error
		}
		reporter.set(&msg)
	}
	return nil
}

func (d *kubeDockerClient) RemoveImage(
	image string,
	opts dockerimagetypes.RemoveOptions,
) ([]dockerimagetypes.DeleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.ImageRemove(ctx, image, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if dockerapi.IsErrNotFound(err) {
		return nil, ImageNotFoundError{ID: image}
	}
	return resp, err
}

func (d *kubeDockerClient) Logs(
	id string,
	opts dockercontainer.LogsOptions,
	sopts StreamOptions,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := d.client.ContainerLogs(ctx, id, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	defer resp.Close()
	return d.redirectResponseToOutputStream(
		sopts.RawTerminal,
		sopts.OutputStream,
		sopts.ErrorStream,
		resp,
	)
}

func (d *kubeDockerClient) Version() (*dockertypes.Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.ServerVersion(ctx)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *kubeDockerClient) Info() (*dockersystem.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.Info(ctx)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *kubeDockerClient) CreateExec(
	id string,
	opts dockercontainer.ExecOptions,
) (*dockertypes.IDResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.ContainerExecCreate(ctx, id, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *kubeDockerClient) StartExec(
	startExec string,
	opts dockercontainer.ExecStartOptions,
	sopts StreamOptions,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if opts.Detach {
		err := d.client.ContainerExecStart(ctx, startExec, opts)
		if ctxErr := contextError(ctx); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	resp, err := d.client.ContainerExecAttach(ctx, startExec, dockercontainer.ExecStartOptions{
		Detach: opts.Detach,
		Tty:    opts.Tty,
	})
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	defer resp.Close()

	if sopts.ExecStarted != nil {
		// Send a message to the channel indicating that the exec has started. This is needed so
		// interactive execs can handle resizing correctly - the request to resize the TTY has to happen
		// after the call to d.client.ContainerExecAttach, and because d.holdHijackedConnection below
		// blocks, we use sopts.ExecStarted to signal the caller that it's ok to resize.
		sopts.ExecStarted <- struct{}{}
	}

	return d.holdHijackedConnection(
		sopts.RawTerminal || opts.Tty,
		sopts.InputStream,
		sopts.OutputStream,
		sopts.ErrorStream,
		resp,
	)
}

func (d *kubeDockerClient) InspectExec(id string) (*dockercontainer.ExecInspect, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	resp, err := d.client.ContainerExecInspect(ctx, id)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *kubeDockerClient) AttachToContainer(
	id string,
	opts dockercontainer.AttachOptions,
	sopts StreamOptions,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := d.client.ContainerAttach(ctx, id, opts)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	defer resp.Close()
	return d.holdHijackedConnection(
		sopts.RawTerminal,
		sopts.InputStream,
		sopts.OutputStream,
		sopts.ErrorStream,
		resp,
	)
}

func (d *kubeDockerClient) ResizeExecTTY(id string, height, width uint) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return d.client.ContainerExecResize(ctx, id, dockercontainer.ResizeOptions{
		Height: height,
		Width:  width,
	})
}

func (d *kubeDockerClient) ResizeContainerTTY(id string, height, width uint) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return d.client.ContainerResize(ctx, id, dockercontainer.ResizeOptions{
		Height: height,
		Width:  width,
	})
}

// GetContainerStats is currently only used for Windows container stats
func (d *kubeDockerClient) GetContainerStats(id string) (*dockercontainer.StatsResponse, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	response, err := d.client.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(response.Body)
	var stats dockercontainer.StatsResponse
	err = dec.Decode(&stats)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	return &stats, nil
}

// redirectResponseToOutputStream redirect the response stream to stdout and stderr. When tty is true, all stream will
// only be redirected to stdout.
func (d *kubeDockerClient) redirectResponseToOutputStream(
	tty bool,
	outputStream, errorStream io.Writer,
	resp io.Reader,
) error {
	if outputStream == nil {
		outputStream = io.Discard
	}
	if errorStream == nil {
		errorStream = io.Discard
	}
	var err error
	if tty {
		_, err = io.Copy(outputStream, resp)
	} else {
		_, err = dockerstdcopy.StdCopy(outputStream, errorStream, resp)
	}
	return err
}

// holdHijackedConnection hold the HijackedResponse, redirect the inputStream to the connection, and redirect the response
// stream to stdout and stderr. NOTE: If needed, we could also add context in this function.
func (d *kubeDockerClient) holdHijackedConnection(
	tty bool,
	inputStream io.Reader,
	outputStream, errorStream io.Writer,
	resp dockertypes.HijackedResponse,
) error {
	receiveStdout := make(chan error)
	if outputStream != nil || errorStream != nil {
		go func() {
			receiveStdout <- d.redirectResponseToOutputStream(tty, outputStream, errorStream, resp.Reader)
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inputStream != nil {
			io.Copy(resp.Conn, inputStream)
		}
		resp.CloseWrite()
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		return err
	case <-stdinDone:
		if outputStream != nil || errorStream != nil {
			return <-receiveStdout
		}
	}
	return nil
}

// getCustomTimeoutContext returns a new context with a specific request timeout
func (d *kubeDockerClient) getCustomTimeoutContext(
	timeout time.Duration,
) (context.Context, context.CancelFunc) {
	// Pick the larger of the two
	if d.timeout > timeout {
		timeout = d.timeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

// contextError checks the context, and returns error if the context is timeout.
func contextError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return operationTimeout{err: ctx.Err()}
	}
	return ctx.Err()
}

// StreamOptions are the options used to configure the stream redirection
type StreamOptions struct {
	RawTerminal  bool
	InputStream  io.Reader
	OutputStream io.Writer
	ErrorStream  io.Writer
	ExecStarted  chan struct{}
}

// operationTimeout is the error returned when the docker operations are timeout.
type operationTimeout struct {
	err error
}

func (e operationTimeout) Error() string {
	return fmt.Sprintf("operation timeout: %v", e.err)
}

func (e operationTimeout) Unwrap() error {
	return e.err
}

// containerNotFoundErrorRegx is the regexp of container not found error message.
var containerNotFoundErrorRegx = regexp.MustCompile(`No such container: [0-9a-z]+`)

// IsContainerNotFoundError checks whether the error is container not found error.
func IsContainerNotFoundError(err error) bool {
	return containerNotFoundErrorRegx.MatchString(err.Error())
}

// ImageNotFoundError is the error returned by InspectImage when image not found.
// Expose this to inject error in cri-dockerd for testing.
type ImageNotFoundError struct {
	ID string
}

func (e ImageNotFoundError) Error() string {
	return fmt.Sprintf("no such image: %q", e.ID)
}

// IsImageNotFoundError checks whether the error is image not found error. This is exposed
// to share with cri-dockerd.
func IsImageNotFoundError(err error) bool {
	_, ok := err.(ImageNotFoundError)
	return ok
}
