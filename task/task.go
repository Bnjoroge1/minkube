package task

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type Task struct {
	ID             uuid.UUID         `json:"id"`
	ContainerID    string            `json:"containerID"`
	Name           string            `json:"name"`
	State          State             `json:"state"`
	Image          string            `json:"image"`
	Memory         int               `json:"memory"`
	Disk           int               `json:"disk"`
	ExposedPortSet nat.PortSet       `json:"exposedPortSet"`
	PortBindings   map[string]string `json:"portBindings"`
	RestartPolicy  string            `json:"restartPolicy"`
	StartTime      time.Time         `json:"startTime"`
	EndTime        time.Time         `json:"endTime"`
}

// docker container config
type Config struct {
	Name          string
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	Cmd           []string
	CPU           float64
	Image         string
	Memory        int64 //used by scheduler to find a node in cluster capableof running task
	Disk          int64
	Env           []string //pass envirobment variaobles to container
	RestartPolicy string   // RestartPolicy for the container if it dies unexpectedly(provides resilienc) ["", "always", "unless-stopped", "on-failure"]

}

func NewConfig(t *Task) *Config {
	log.Printf("NewConfig: Creating config for task %v", t.ID)

	config := &Config{
		Name:  t.Name,
		Image: t.Image,

		RestartPolicy: t.RestartPolicy,
	}
	log.Printf("NewConfig: Created config for task %v", t.ID)
	return config
}

// mapping task to docker container
type Docker struct {
	Client      *client.Client
	Config      Config
	ContainerId string
}

func NewDocker(c *Config) *Docker {
	log.Printf("NewDocker: Creating Docker client")
	dc, _ := client.NewClientWithOpts(client.FromEnv)
	if dc == nil {
		log.Printf("NewDocker: Failed to create Docker client")
		return nil
	}
	log.Printf("NewDocker: Created Docker client")
	return &Docker{
		Client: dc,
		Config: *c,
	}
}

// result of Docker operation
type DockerResult struct {
	Error       error
	Action      string //start, create etc
	ContainerId string
	Result      string
}

// TaskEvent is a struct to represent the user's desire to change the status from one to the other.
type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}

// run container
func (d *Docker) Run() DockerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(
		ctx,
		d.Config.Image,
		types.ImagePullOptions{})
	if err != nil {
		log.Printf("Error pulling image %s:%v \n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	io.Copy(os.Stdout, reader)

	//get config info
	rp := container.RestartPolicy{
		Name: d.Config.RestartPolicy,
	}
	r := container.Resources{
		Memory: d.Config.Memory,
	}
	cc := container.Config{
		Image: d.Config.Image,
		Env:   d.Config.Env,
	}
	hc := container.HostConfig{
		RestartPolicy:   rp,
		Resources:       r,
		PublishAllPorts: true,
	}
	//attempts to create container
	resp, err := d.Client.ContainerCreate(
		ctx, &cc, &hc, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("Error creating container using image %s: %v \n",
			d.Config.Image, err)
		return DockerResult{Error: err}
	}

	//start container
	err = d.Client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("Error starting container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	d.ContainerId = resp.ID
	out, err := d.Client.ContainerLogs(
		ctx,
		resp.ID,
		types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true},
	)
	if err != nil {
		log.Printf("Error getting container logs %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	if err != nil {
		log.Printf("Error copying container logs to stdout/stderr: %v\n", err)
		return DockerResult{Error: err}
	}

	return DockerResult{
		ContainerId: resp.ID,
		Action:      "start",
		Result:      "success",
	}

}

func (d *Docker) Stop(id string) DockerResult {
	log.Printf("Attempting to stop container %v", id)
	ctx := context.Background()
	err := d.Client.ContainerStop(ctx, id, nil)
	if err != nil {
		return DockerResult{
			Action: "stop",
			Result: "failed",
			Error:  err,
		}
	}
	err = d.Client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{})
	if err != nil {
		return DockerResult{
			Action: "stop",
			Result: "failed",
			Error:  err,
		}
	}
	return DockerResult{Action: "stop", Result: "success", Error: nil}
}

// helper to check if task is running
func (d *Docker) IsRunning(containerID string) (bool, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return false, err
	}
	defer cli.Close()
	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	return container.State.Running, nil
}

func ValidateStateTransition(src State, dst State) bool {
	return containsState(stateTransitionMap[src], dst)
}
