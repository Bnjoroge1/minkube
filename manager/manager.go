package manager

import (
	"fmt"
	"minkube/task"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)


type Manager struct {
	Pending queue.Queue
	TaskDb map[string][]task.Task
	EventDb map[string][]task.TaskEvent
	Workers []string
	WorkersTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
}

//schedule task to workers
//given a task, evaluate all resources available in pool of workers to find suitable worker. 
func (m* Manager) SelectWorker () {
	fmt.Println("I wil select an appropriate worker to send tasks to")
}

//updates the status of tasks
func (m* Manager) UpdateTasks() {
	fmt.Println(("I will update tasks"))
}

//sends tasks to workers
func (m* Manager) SendWork() {
	fmt.Println("I will send work to workers")
}