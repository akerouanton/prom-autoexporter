package backend_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/NiR-/prom-autoexporter/backend"
	"github.com/NiR-/prom-autoexporter/log"
	"github.com/NiR-/prom-autoexporter/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"gotest.tools/assert"
)

type fakeFn int

const (
	imagePullFn fakeFn = iota
	containerCreateFn
	containerStartFn
	containerStopFn
	containerListFn
	containerInspectFn
	containerRemoveFn
	networkConnectFn
	networkDisconnectFn
	networkInspectFn
	taskListFn
	serviceInspectFn
	eventsFn
)

type fakeCall struct {
	t            *testing.T
	callsCounter uint
}

type fakeClient struct {
	client.Client

	t         *testing.T
	fakeCalls map[fakeFn]*fakeCall

	imagePullFn func(*fakeCall, context.Context, string, types.ImagePullOptions) (io.ReadCloser, error)

	containerCreateFn  func(*fakeCall, context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, string) (container.ContainerCreateCreatedBody, error)
	containerStartFn   func(*fakeCall, context.Context, string, types.ContainerStartOptions) error
	containerStopFn    func(*fakeCall, context.Context, string, *time.Duration) error
	containerListFn    func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error)
	containerInspectFn func(*fakeCall, context.Context, string) (types.ContainerJSON, error)
	containerRemoveFn  func(*fakeCall, context.Context, string, types.ContainerRemoveOptions) error

	networkConnectFn    func(*fakeCall, context.Context, string, string, *network.EndpointSettings) error
	networkDisconnectFn func(*fakeCall, context.Context, string, string, bool) error
	networkInspectFn    func(*fakeCall, context.Context, string, types.NetworkInspectOptions) (types.NetworkResource, error)

	taskListFn       func(*fakeCall, context.Context, types.TaskListOptions) ([]swarm.Task, error)
	serviceInspectFn func(*fakeCall, context.Context, string, types.ServiceInspectOptions) (swarm.Service, []byte, error)

	eventsFn func(*fakeCall, context.Context, types.EventsOptions) (<-chan events.Message, <-chan error)
}

func (c *fakeClient) findFakeCall(fn fakeFn) *fakeCall {
	if c.fakeCalls == nil {
		c.fakeCalls = map[fakeFn]*fakeCall{}
	}
	if _, ok := c.fakeCalls[fn]; !ok {
		c.fakeCalls[fn] = &fakeCall{c.t, 0}
	}
	return c.fakeCalls[fn]
}

func (c *fakeClient) ImagePull(ctx context.Context, image string, opts types.ImagePullOptions) (io.ReadCloser, error) {
	if c.imagePullFn != nil {
		fc := c.findFakeCall(imagePullFn)
		return c.imagePullFn(fc, ctx, image, opts)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
}

func (c *fakeClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, netConfig *network.NetworkingConfig, name string) (container.ContainerCreateCreatedBody, error) {
	if c.containerCreateFn != nil {
		fc := c.findFakeCall(containerCreateFn)
		return c.containerCreateFn(fc, ctx, config, hostConfig, netConfig, name)
	}
	return container.ContainerCreateCreatedBody{ID: "9d234f"}, nil
}

func (c *fakeClient) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	if c.networkConnectFn != nil {
		fc := c.findFakeCall(networkConnectFn)
		return c.networkConnectFn(fc, ctx, networkID, containerID, config)
	}
	return nil
}

func (c *fakeClient) ContainerStart(ctx context.Context, containerID string, opts types.ContainerStartOptions) error {
	if c.containerStartFn != nil {
		fc := c.findFakeCall(containerStartFn)
		return c.containerStartFn(fc, ctx, containerID, opts)
	}
	return nil
}

func (c *fakeClient) ContainerList(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
	if c.containerListFn != nil {
		fc := c.findFakeCall(containerListFn)
		return c.containerListFn(fc, ctx, opts)
	}
	return []types.Container{}, nil
}

func (c *fakeClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	fc := c.findFakeCall(containerInspectFn)
	fc.callsCounter++
	if c.containerInspectFn != nil {
		return c.containerInspectFn(fc, ctx, containerID)
	}
	return types.ContainerJSON{}, nil
}

func (c *fakeClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	fc := c.findFakeCall(networkDisconnectFn)
	fc.callsCounter++
	if c.networkDisconnectFn != nil {
		return c.networkDisconnectFn(fc, ctx, networkID, containerID, force)
	}
	return nil
}

func (c *fakeClient) ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error {
	fc := c.findFakeCall(containerStopFn)
	fc.callsCounter++
	if c.containerStopFn != nil {
		return c.containerStopFn(fc, ctx, containerID, timeout)
	}
	return nil
}

func (c *fakeClient) ContainerRemove(ctx context.Context, containerID string, opts types.ContainerRemoveOptions) error {
	fc := c.findFakeCall(containerRemoveFn)
	fc.callsCounter++
	if c.containerRemoveFn != nil {
		return c.containerRemoveFn(fc, ctx, containerID, opts)
	}
	return nil
}

func (c *fakeClient) NetworkInspect(ctx context.Context, networkID string, opts types.NetworkInspectOptions) (types.NetworkResource, error) {
	if c.networkInspectFn != nil {
		fc := c.findFakeCall(networkInspectFn)
		return c.networkInspectFn(fc, ctx, networkID, opts)
	}
	return types.NetworkResource{}, nil
}

func (c *fakeClient) TaskList(ctx context.Context, opts types.TaskListOptions) ([]swarm.Task, error) {
	if c.taskListFn != nil {
		fc := c.findFakeCall(taskListFn)
		return c.taskListFn(fc, ctx, opts)
	}
	return []swarm.Task{}, nil
}

func (c *fakeClient) ServiceInspectWithRaw(ctx context.Context, serviceID string, opts types.ServiceInspectOptions) (swarm.Service, []byte, error) {
	if c.serviceInspectFn != nil {
		fc := c.findFakeCall(serviceInspectFn)
		fc.callsCounter++
		return c.serviceInspectFn(fc, ctx, serviceID, opts)
	}
	return swarm.Service{}, []byte{}, nil
}

func (c *fakeClient) Events(ctx context.Context, opts types.EventsOptions) (<-chan events.Message, <-chan error) {
	if c.eventsFn != nil {
		fc := c.findFakeCall(eventsFn)
		return c.eventsFn(fc, ctx, opts)
	}
	evtCh := make(<-chan events.Message)
	errCh := make(<-chan error)
	return evtCh, errCh
}

func TestRunExporter(t *testing.T) {
	testcases := map[string]struct {
		cli           *fakeClient
		expectedError string
	}{
		"successful": {
			cli: &fakeClient{
				imagePullFn: func(fc *fakeCall, ctx context.Context, image string, opts types.ImagePullOptions) (io.ReadCloser, error) {
					assert.Equal(fc.t, image, "oliver006/redis_exporter:latest")
					return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
				},
				containerCreateFn: func(fc *fakeCall, ctx context.Context, config *container.Config, hostConfig *container.HostConfig, netConfig *network.NetworkingConfig, name string) (container.ContainerCreateCreatedBody, error) {
					assert.Equal(fc.t, config.Image, "oliver006/redis_exporter:latest")
					assert.DeepEqual(fc.t, config.Cmd, strslice.StrSlice{"-redis.addr=redis://localhost:6379"})
					assert.DeepEqual(fc.t, config.Env, []string{"FOO=BAR"})
					assert.Equal(fc.t, hostConfig.NetworkMode, container.NetworkMode("container:012dfc9"))
					return container.ContainerCreateCreatedBody{ID: "9d234f"}, nil
				},
				networkConnectFn: func(fc *fakeCall, ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
					assert.Equal(fc.t, networkID, "testnet")
					assert.Equal(fc.t, containerID, "012dfc9")
					return nil
				},
				containerStartFn: func(fc *fakeCall, ctx context.Context, containerID string, opts types.ContainerStartOptions) error {
					assert.Equal(fc.t, containerID, "9d234f")
					return nil
				},
			},
			expectedError: "",
		},
		"pulling image failed": {
			cli: &fakeClient{
				imagePullFn: func(*fakeCall, context.Context, string, types.ImagePullOptions) (io.ReadCloser, error) {
					return nil, errors.New("error pulling image")
				},
			},
			expectedError: "error pulling image",
		},
		"creating contaner failed": {
			cli: &fakeClient{
				containerCreateFn: func(*fakeCall, context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, string) (container.ContainerCreateCreatedBody, error) {
					return container.ContainerCreateCreatedBody{}, errors.New("error creating container")
				},
			},
			expectedError: "error creating container",
		},
		"connecting to network failed": {
			cli: &fakeClient{
				networkConnectFn: func(*fakeCall, context.Context, string, string, *network.EndpointSettings) error {
					return errors.New("error connecting to network")
				},
			},
			expectedError: "error connecting to network",
		},
		"starting container failed": {
			cli: &fakeClient{
				containerStartFn: func(*fakeCall, context.Context, string, types.ContainerStartOptions) error {
					return errors.New("error starting container")
				},
			},
			expectedError: "error starting container",
		},
	}

	for tcname, _ := range testcases {
		t.Run(tcname, func(t *testing.T) {
			tc := testcases[tcname]
			tc.cli.t = t
			t.Parallel()

			ctx := context.Background()
			exporter := models.Exporter{
				Name:         "exporter004",
				ExporterType: "redis",
				Image:        "oliver006/redis_exporter:latest",
				Cmd:          []string{"-redis.addr=redis://localhost:6379"},
				EnvVars:      []string{"FOO=BAR"},
				Port:         "9121",
				ExportedTask: models.TaskToExport{
					ID:     "012dfc9",
					Name:   "task-to-export",
					Labels: map[string]string{},
				},
			}

			f := models.NewPredefinedExporterFinder()
			b := backend.NewDockerBackend(tc.cli, "testnet", f)
			err := b.RunExporter(ctx, exporter)

			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestCancelRunExporter(t *testing.T) {
	ctx, cancelfunc := context.WithCancel(context.Background())
	cli := &fakeClient{
		t: t,
		containerCreateFn: func(*fakeCall, context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, string) (container.ContainerCreateCreatedBody, error) {
			cancelfunc()
			return container.ContainerCreateCreatedBody{}, nil
		},
		networkConnectFn: func(fc *fakeCall, _ context.Context, _ string, _ string, _ *network.EndpointSettings) error {
			assert.Assert(fc.t, false, "NetworkConnect should not be called")
			return nil
		},
	}
	exporter := models.Exporter{
		Name:         "exporter004",
		ExporterType: "redis",
		Image:        "oliver006/redis_exporter:latest",
		Cmd:          []string{"-redis.addr=redis://localhost:6379"},
		EnvVars:      []string{"FOO=BAR"},
		Port:         "9121",
		ExportedTask: models.TaskToExport{
			ID:     "012dfc9",
			Name:   "task-to-export",
			Labels: map[string]string{},
		},
	}
	f := models.NewPredefinedExporterFinder()
	b := backend.NewDockerBackend(cli, "testnet", f)
	err := b.RunExporter(ctx, exporter)

	assert.NilError(t, err)
}

func TestCleanupExporter(t *testing.T) {
	testcases := map[string]struct {
		cli           *fakeClient
		exporterName  string
		forceCleanup  bool
		expectedError string
	}{
		"succeeds to forcefully cleanup when exported task's still running": {
			cli: &fakeClient{
				containerListFn: func(fc *fakeCall, ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
					assert.Assert(fc.t, opts.Filters.ExactMatch("name", "exporter001"))
					return []types.Container{
						{
							ID:    "exporter-cid",
							Names: []string{"exporter001"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(fc *fakeCall, ctx context.Context, containerID string) (types.ContainerJSON, error) {
					assert.Equal(fc.t, containerID, "exported-task-cid")
					return testNewContainerJSON("exported-task-cid", &types.ContainerState{Running: true}), nil
				},
				networkDisconnectFn: func(fc *fakeCall, ctx context.Context, networkID string, containerID string, force bool) error {
					assert.Equal(fc.t, networkID, "testnet")
					assert.Equal(fc.t, containerID, "exporter-cid")
					assert.Equal(fc.t, force, true)
					return nil
				},
				containerStopFn: func(fc *fakeCall, ctx context.Context, containerID string, timeout *time.Duration) error {
					assert.Equal(fc.t, containerID, "exporter-cid")
					return nil
				},
				containerRemoveFn: func(fc *fakeCall, ctx context.Context, containerID string, opts types.ContainerRemoveOptions) error {
					assert.Equal(fc.t, containerID, "exporter-cid")
					assert.Equal(fc.t, opts.Force, true)
					return nil
				},
			},
			exporterName:  "exporter001",
			forceCleanup:  true,
			expectedError: "",
		},
		"fails to cleanup when exported task's still running": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID: "exporter-cid",
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(fc *fakeCall, ctx context.Context, containerID string) (types.ContainerJSON, error) {
					assert.Equal(fc.t, containerID, "exported-task-cid")
					return testNewContainerJSON("exported-task-cid", &types.ContainerState{Running: true}), nil
				},
			},
			exporterName:  "exporter002",
			forceCleanup:  false,
			expectedError: "Exporter \"exporter-cid\" can't be stopped, exported container \"exported-task-cid\" still running.",
		},
		"does not find the exporter to cleanup": {
			cli:           &fakeClient{},
			exporterName:  "exporter003",
			forceCleanup:  false,
			expectedError: "exporter not found",
		},
		"fails to inspect the exported container": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID: "exporter-cid",
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(*fakeCall, context.Context, string) (types.ContainerJSON, error) {
					return types.ContainerJSON{}, errors.New("error inspecting container")
				},
			},
			exporterName:  "exporter004",
			forceCleanup:  false,
			expectedError: "error inspecting container",
		},
		// @TODO: check what happens when the exporter isn't connected to the network
		/* "fails to disconnect the exporter": {
			cli: &fakeClient{
				containerListFn: func(ctx context.Context, opts types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID: "exporter-cid",
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(fc *fakeCall, ctx context.Context, containerID string) (types.ContainerJSON, error) {
					return types.ContainerJSON{}, fakeNotFoundError{}
				},
				networkDisconnectFn: func(fc *fakeCall, ctx context.Context, networkID string, containerID string, force bool) error {
					assert.Equal(t, force, false)
					return errors.New("error disconnecting from network")
				},
			},
			exporterName:  "exporter005",
			forceCleanup:  false,
			expectedError: "error disconnecting from network",
		}, */
		// @TODO: check what happens when the exporter isn't running
		"fails to stop the exporter": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID: "exporter-cid",
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(*fakeCall, context.Context, string) (types.ContainerJSON, error) {
					return types.ContainerJSON{}, fakeNotFoundError{}
				},
				containerStopFn: func(*fakeCall, context.Context, string, *time.Duration) error {
					return errors.New("error stopping container")
				},
			},
			exporterName:  "exporter006",
			forceCleanup:  false,
			expectedError: "error stopping container",
		},
		"fails to remove the exporter": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID:    "exporter-cid",
							Names: []string{"exporter007"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(*fakeCall, context.Context, string) (types.ContainerJSON, error) {
					return types.ContainerJSON{}, fakeNotFoundError{}
				},
				containerRemoveFn: func(fc *fakeCall, ctx context.Context, containerID string, opts types.ContainerRemoveOptions) error {
					assert.Equal(fc.t, opts.Force, false)
					return errors.New("error removing container")
				},
			},
			exporterName:  "exporter007",
			forceCleanup:  false,
			expectedError: "error removing container",
		},
	}

	for tcname, _ := range testcases {
		t.Run(tcname, func(t *testing.T) {
			tc := testcases[tcname]
			tc.cli.t = t
			t.Parallel()

			ctx := context.Background()
			f := models.NewPredefinedExporterFinder()
			b := backend.NewDockerBackend(tc.cli, "testnet", f)
			err := b.CleanupExporter(ctx, tc.exporterName, tc.forceCleanup)

			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)
		})
	}
}

type fakeNotFoundError struct{}

func (e fakeNotFoundError) NotFound() bool {
	return true
}

func (e fakeNotFoundError) Error() string {
	return "fake not found error"
}

func testNewContainerJSON(containerID string, state *types.ContainerState) types.ContainerJSON {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    containerID,
			State: state,
		},
	}
}

func TestCleanupExporters(t *testing.T) {
	testcases := map[string]struct {
		cli           *fakeClient
		forceCleanup  bool
		expectedError string
	}{
		"suceeds to forcefully cleanup exporters": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID:    "exporter001-cid",
							Names: []string{"exporter001"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task001-cid",
							},
						},
						{
							ID:    "exporter002-cid",
							Names: []string{"exporter002"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task002-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(fc *fakeCall, ctx context.Context, containerID string) (types.ContainerJSON, error) {
					exportedCID := fmt.Sprintf("exported-task%03d-cid", fc.callsCounter)
					assert.Equal(fc.t, containerID, exportedCID)
					return testNewContainerJSON(exportedCID, &types.ContainerState{Running: true}), nil
				},
				networkDisconnectFn: func(fc *fakeCall, ctx context.Context, networkID string, containerID string, force bool) error {
					exportedCID := fmt.Sprintf("exporter%03d-cid", fc.callsCounter)
					assert.Equal(fc.t, containerID, exportedCID)
					assert.Equal(fc.t, networkID, "testnet")
					assert.Equal(fc.t, force, true)
					return nil
				},
				containerStopFn: func(fc *fakeCall, ctx context.Context, containerID string, timeout *time.Duration) error {
					exportedCID := fmt.Sprintf("exporter%03d-cid", fc.callsCounter)
					assert.Equal(fc.t, containerID, exportedCID)
					return nil
				},
				containerRemoveFn: func(fc *fakeCall, ctx context.Context, containerID string, opts types.ContainerRemoveOptions) error {
					exportedCID := fmt.Sprintf("exporter%03d-cid", fc.callsCounter)
					assert.Equal(fc.t, containerID, exportedCID)
					assert.Equal(fc.t, opts.Force, true)
					return nil
				},
			},
			forceCleanup:  true,
			expectedError: "",
		},
		"cleanups what it can and fails for tasks still running": {
			cli: &fakeClient{
				containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
					return []types.Container{
						{
							ID:    "exporter001-cid",
							Names: []string{"exporter001"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task001-cid",
							},
						},
						{
							ID:    "exporter002-cid",
							Names: []string{"exporter002"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task002-cid",
							},
						},
						{
							ID:    "exporter003-cid",
							Names: []string{"exporter003"},
							Labels: map[string]string{
								backend.LABEL_EXPORTED_ID: "exported-task003-cid",
							},
						},
					}, nil
				},
				containerInspectFn: func(fc *fakeCall, ctx context.Context, containerID string) (types.ContainerJSON, error) {
					exportedCID := fmt.Sprintf("exported-task%03d-cid", fc.callsCounter)
					assert.Equal(fc.t, containerID, exportedCID)
					if fc.callsCounter == 2 {
						return types.ContainerJSON{}, fakeNotFoundError{}
					}
					return testNewContainerJSON(exportedCID, &types.ContainerState{Running: true}), nil
				},
				networkDisconnectFn: func(fc *fakeCall, ctx context.Context, networkID string, containerID string, force bool) error {
					assert.Equal(fc.t, containerID, "exporter002-cid", "NetworkDisconneect: expected \"exporter002-cid\", got ")
					assert.Equal(fc.t, networkID, "testnet")
					assert.Equal(fc.t, force, false)
					return nil
				},
				containerStopFn: func(fc *fakeCall, ctx context.Context, containerID string, timeout *time.Duration) error {
					assert.Equal(fc.t, containerID, "exporter002-cid")
					return nil
				},
				containerRemoveFn: func(fc *fakeCall, ctx context.Context, containerID string, opts types.ContainerRemoveOptions) error {
					assert.Equal(fc.t, containerID, "exporter002-cid")
					assert.Equal(fc.t, opts.Force, false)
					return nil
				},
			},
			forceCleanup:  false,
			expectedError: "",
		},
	}

	for tcname, _ := range testcases {
		t.Run(tcname, func(t *testing.T) {
			tc := testcases[tcname]
			tc.cli.t = t
			t.Parallel()

			ctx := context.Background()
			f := models.NewPredefinedExporterFinder()
			b := backend.NewDockerBackend(tc.cli, "testnet", f)
			err := b.CleanupExporters(ctx, tc.forceCleanup)

			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			assert.NilError(t, err)
		})
	}
}

func TestFindMissingExporters(t *testing.T) {
	cli := &fakeClient{
		containerListFn: func(*fakeCall, context.Context, types.ContainerListOptions) ([]types.Container, error) {
			return []types.Container{
				{
					ID:     "exported-task001-cid",
					Names:  []string{"/exported-task001"},
					Labels: map[string]string{},
				},
				{
					ID:    "exporter001-cid",
					Names: []string{"/exporter.type.exported-task001"},
					Labels: map[string]string{
						backend.LABEL_EXPORTED_ID:   "exported-task001-cid",
						backend.LABEL_EXPORTED_NAME: "exported-task001",
					},
				},
				{
					ID:     "exported-task002-cid",
					Names:  []string{"/redis"},
					Labels: map[string]string{},
				},
			}, nil
		},
	}

	f := fakeExporterFinder{
		findMatchingExportersFn: func(t models.TaskToExport) map[string]models.Exporter {
			name := "type"
			image := "some/image"
			port := "8080"

			if t.Name == "/redis" {
				name = "redis"
				image = "oliver006/redis_exporter:v0.25.0"
				port = "9121"
			}

			exporter, _ := models.NewExporter(name, name, image, []string{}, []string{}, port, t)

			return map[string]models.Exporter{
				name: exporter,
			}
		},
	}

	b := backend.NewDockerBackend(cli, "testnet", f)
	missing, err := b.FindMissingExporters(context.Background())
	assert.NilError(t, err)

	assert.DeepEqual(t, missing, []models.Exporter{
		{
			Name:         "/exporter.redis.redis",
			ExporterType: "redis",
			Image:        "oliver006/redis_exporter:v0.25.0",
			Cmd:          []string{},
			EnvVars:      []string{},
			Port:         "9121",
			ExportedTask: models.TaskToExport{
				ID:     "exported-task002-cid",
				Name:   "/redis",
				Labels: map[string]string{},
			},
		},
	})
}

type fakeExporterFinder struct {
	findMatchingExportersFn func(t models.TaskToExport) map[string]models.Exporter
}

func (f fakeExporterFinder) FindMatchingExporters(t models.TaskToExport) (map[string]models.Exporter, []error) {
	return f.findMatchingExportersFn(t), []error{}
}

func TestListenForTasksToExport(t *testing.T) {
	log.ConfigureDefaultLogger("debug")

	testcases := map[string]struct {
		dockerEvent   events.Message
		expectedEvent models.TaskEvent
	}{
		"sends a task started event when a docker container starts": {
			dockerEvent: events.Message{
				Action: "start",
				Actor: events.Actor{
					ID: "/container-to-export-cid",
					Attributes: map[string]string{
						"name":       "container-to-export",
						"image":      "some/image",
						"foo":        "bar",
						"some.label": "bzzzz",
					},
				},
			},
			expectedEvent: models.TaskEvent{
				Task: models.TaskToExport{
					ID:   "/container-to-export-cid",
					Name: "container-to-export",
					Labels: map[string]string{
						"foo":        "bar",
						"some.label": "bzzzz",
					},
				},
				Type: models.TaskStarted,
				Exporters: []models.Exporter{
					{
						Name:         "/exporter.redis.container-to-export",
						ExporterType: "redis",
					},
				},
			},
		},
		"sends a task stopped event when a constainer die": {
			dockerEvent: events.Message{
				Action: "die",
				Actor: events.Actor{
					ID: "/container-to-export-cid",
					Attributes: map[string]string{
						"name":       "container-to-export",
						"image":      "some/image",
						"exitCode":   "137",
						"foo":        "bar",
						"some.label": "bzzzz",
					},
				},
			},
			expectedEvent: models.TaskEvent{
				Task: models.TaskToExport{
					ID:   "/container-to-export-cid",
					Name: "container-to-export",
					Labels: map[string]string{
						"foo":        "bar",
						"some.label": "bzzzz",
					},
				},
				Type: models.TaskStopped,
				Exporters: []models.Exporter{
					{
						Name:         "/exporter.redis.container-to-export",
						ExporterType: "redis",
					},
				},
			},
		},
	}

	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			cli := newFakeEventsListener([]events.Message{tc.dockerEvent})
			f := &fakeExporterFinder{
				findMatchingExportersFn: func(t models.TaskToExport) map[string]models.Exporter {
					return map[string]models.Exporter{
						"redis": {
							ExporterType: "redis",
						},
					}
				},
			}
			b := backend.NewDockerBackend(cli, "", f)

			ctx := context.Background()
			taskEvtCh := make(chan models.TaskEvent)
			go b.ListenForTasksToExport(ctx, taskEvtCh)

			timer := time.NewTimer(1 * time.Second)
			for {
				select {
				case <-timer.C:
					t.Error("Test timed out.")
					return
				case received := <-taskEvtCh:
					assert.DeepEqual(t, received, tc.expectedEvent)
					return
				}
			}
		})
	}
}

func newFakeEventsListener(evts []events.Message) *fakeClient {
	return &fakeClient{
		eventsFn: func(*fakeCall, context.Context, types.EventsOptions) (<-chan events.Message, <-chan error) {
			evtCh := make(chan events.Message)
			errCh := make(chan error)

			go func() {
				for _, evt := range evts {
					evtCh <- evt
				}
			}()

			return evtCh, errCh
		},
	}
}