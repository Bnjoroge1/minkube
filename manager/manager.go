package manager

import (
	"fmt"
	"minkube/task"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)


type Manager struct {
	Pending queue.Queue
	TaskDb map[uuid.UUID]task.Task
	EventDb map[string][]task.TaskEvent
	Workers []string
	WorkersTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
}

//Get task from db 	
func (m *Manager) GetTask(id uuid.UUID) (task.Task, bool) {
	task, exists := m.TaskDb[id]
	return task, exists
 }

 //update task in db
func (m *Manager) UpdateTaskState(id uuid.UUID, state task.State) {
	if task, exists := m.TaskDb[id]; exists {
	    task.State = state
	    m.TaskDb[id] = task
	}
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