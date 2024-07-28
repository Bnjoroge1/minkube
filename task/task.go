package task

import (
	"time"

	//"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
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
	//ExposedPortSet nat.PortSet //ports for networking
	//PortBindings map[string]string  //maps container ports to host ports
	//RestartPolicy string //how container should be restarted if it stops
	//StartTime  time.Time
	//EndTime time.Time
	//Command []string

}

//TaskEvent is a struct to represent the user's desire to change the status from one to the other.
type TaskEvent struct {
	ID      uuid.UUID
	State State
	Timestamp time.Time
	Task Task
 }