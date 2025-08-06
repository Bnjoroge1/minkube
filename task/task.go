package task

import (
	"context"
	"io"
	"log"
	//"minkube/task"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
	HostPorts      nat.PortMap       `json:"hostPorts"` //ports assigned by docker
	PortBindings   map[string]string `json:"portBindings"`
	RestartPolicy  string            `json:"restartPolicy"`
	StartTime      time.Time         `json:"startTime"`
	EndTime        time.Time         `json:"endTime"`
	HealthCheck string
	RestartCount int
}
type DockerInspectResponse struct {
	Error error
	Container *types.ContainerJSON
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

func (task *Task) NewConfig(t *Task) *Config {
	log.Printf("NewConfig: Creating config for task %v", t.ID)

	config := &Config{
		Name:  t.Name,
		Image: t.Image,
		Memory: int64(t.Memory),
		Disk: int64(t.Disk),
		RestartPolicy: t.RestartPolicy,
	}
	log.Printf("NewConfig: Created config for task %v", t.ID)
	return config
}

// mapping task to docker container
type Docker struct {
	Config      Config
	ContainerId string
}

func (task *Task) NewDocker(c *Config) *Docker {
	log.Printf("NewDocker: Creating Docker client")
	dc, _ := client.NewClientWithOpts(client.FromEnv)
	if dc == nil {
		log.Printf("NewDocker: Failed to create Docker client")
		return nil
	}
	log.Printf("NewDocker: Created Docker client")
	return &Docker{
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
	ID        uuid.UUID          `json:"id"`
	State     State              `json:"state"`
	Timestamp time.Time          `json:"timestamp"`
	Task      Task              `json:"task"`
}

// run container
func (d *Docker) Run(client *client.Client) DockerResult {
	ctx := context.Background()
	reader, err := client.ImagePull(
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
	resp, err := client.ContainerCreate(
		ctx, &cc, &hc, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("Error creating container using image %s: %v \n",
			d.Config.Image, err)
		return DockerResult{Error: err}
	}

	//start container
	err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("Error starting container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	d.ContainerId = resp.ID
	
	if err != nil {
		log.Printf("Error getting container logs %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	

	return DockerResult{
		ContainerId: resp.ID,
		Action:      "start",
		Result:      "success",
	}

}

func (d *Docker) Stop(client *client.Client, id string) DockerResult {
	log.Printf("Attempting to stop container %v", id)
	ctx := context.Background()
	err := client.ContainerStop(ctx, id, nil)
	if err != nil {
		return DockerResult{
			Action: "stop",
			Result: "failed",
			Error:  err,
		}
	}
	err = client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{})
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
func (d *Docker) IsRunning(client *client.Client, containerID string) (bool, error) {
	ctx := context.Background()
	container, err := client.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}
	return container.State.Running, nil
}

func (d *Docker) InspectContainer(client *client.Client, containerID string) DockerInspectResponse {
	
	ctx := context.Background()
	response, err := client.ContainerInspect(ctx, containerID)
	if err != nil{
		log.Printf("Error inspecting the container: %s, %v", containerID, err)
		return DockerInspectResponse{Error: err}
	}
	return DockerInspectResponse{
		Container: &response,
	}
}