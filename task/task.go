package task

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/moby/moby/client"
)

type State int

const ( 
	Pending State = iota
	Scheduled
	Running
	Completed 
	Failed 
)

type Task struct {
	ID uuid.UUID
	Name string
	State State
	Image string //docker image
	Memory int //docker container memory allocated
	Disk int //disk is storage allocated
	ExposedPortSet nat.PortSet //ports for networking
	PortBindings map[string]string  //maps container ports to host ports
	RestartPolicy string //how container should be restarted if it stops
	StartTime  time.Time
	EndTime time.Time
	//Command []string
}


//config struct
type Config struct {
	Name string
	AttachStdin bool
	AttachStdout bool
	AttachStderr bool
	Cmd []string
	Image string
	Memory int64
 	Disk int64
	Env []string
	RestartPolicy string   	// RestartPolicy for the container ["", "always", "unless-stopped", "on-failure"]

}
//mapping task to docker container
type Docker struct {
	    Client *client.Client
	    Config  Config
	    ContainerId string
 }
 //result of Docker operation
 type DockerResult struct {
	Error       error
	Action      string  //start, create etc
	ContainerId string
	Result      string
}
//TaskEvent is a struct to represent the user's desire to change the status from one to the other.
type TaskEvent struct {
	ID      uuid.UUID
	State State
	Timestamp time.Time
	Task Task
 }

 //run container
 func( d *Docker) Run() DockerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(
		ctx,
		d.Config.Name,
		types.ImagePullOptions{})	
	if err != nil {
		log.Printf("Error pull image %s:%v \n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	io.Copy(os.Stdout, reader)

	rp := container.RestartPolicy{
		Name: d.Config.RestartPolicy,
	}

	r := container.Resources{
		Memory: d.Config.Memory,
	}

	cc := container.Config{
		Image:        d.Config.Image,
		Tty:          false,
		Env:          d.Config.Env,
		ExposedPorts: d.Config.ExposedPorts,
	}
 }