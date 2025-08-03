package manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"minkube/task"
	"minkube/worker"
	"net/http"
	"sync"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"golang.org/x/tools/go/analysis/passes/defers"
)

type Manager struct {
	Pending        queue.Queue
	TaskDb         map[uuid.UUID]*task.Task
	EventDb        map[uuid.UUID]*task.TaskEvent
	Workers        []string
	WorkersTaskMap map[string][]uuid.UUID
	TaskWorkerMap  map[uuid.UUID]string
	lastWorker     int //track last worker to send task to
	mu         sync.Mutex 
		

}

// Get task from db
func (m *Manager) GetTask(id uuid.UUID) (*task.Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.TaskDb[id]
	return task, exists
}

//update task in db
func (m *Manager) UpdateTaskState(id uuid.UUID, state task.State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, exists := m.TaskDb[id]; exists {
		task.State = state
		m.TaskDb[id] = task
	}
}
func (m *Manager)  AddTask(id uuid.UUID, task *task.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	//adds task to manager
	m.Pending.Enqueue(task)
}
// schedule task to workers
// given a task, evaluate all resources available in pool of workers to find suitable worker.
func (m *Manager) SelectWorker() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	log.Printf("selecting workers")
	if len(m.Workers) == 0 {
		return "", fmt.Errorf("no workers available to select")
	}
	m.lastWorker = (m.lastWorker + 1) % len(m.Workers)
	return m.Workers[m.lastWorker], nil
}

// updates the status of tasks
func (m *Manager) UpdateTasks() {
	
}

// sends tasks to workers
func (m *Manager) SendWork() {
	log.Printf("Sending tasks to workers")
	if m.Pending.Len() > 0 {
		//if there's taks that are still pending. 
		//select worker to run task
		w := m.SelectWorker()
		e := m.Pending.Dequeue()
		te, ok := e.(task.TaskEvent)
		if !ok {
			fmt.Errorf("Task %s is not a task event isntance", e)
		}
		t := te.Task
		log.Printf("Pulled %v task off pending queue")
		m.EventDb[te.ID] = &te  //matching task to event db
		m.WorkersTaskMap[w] = append(m.WorkersTaskMap[w], t.ID)
		m.TaskWorkerMap[t.ID] = w
		t.State = task.Scheduled
		m.TaskDb[t.ID] = &t
		data, err := json.Marshal(&te)
		if err != nil {
			log.Printf("Unable to marshal task object: %v.", t)
		}
		url := fmt.Sprintf("http://%s/tasks", w)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			log.Printf("Error connecting to %v: %v", w, err)
			m.Pending.Enqueue(t)
			return
		}
		d := json.NewDecoder(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			e := worker.ErrResponse{}
			err := d.Decode(&e)
			if err != nil {
				fmt.Printf("Error decoding response: %s\n", e.Message)
				return
			}
			log.Printf("Response error (%d): %s", e.HTTPStatusCode)
			return
			}

			t = task.Task{}
			err = d.Decode(&t)
			if err != nil {
				log.Printf("Error decoding task: %v", err)
				return
			}
			log.Printf("task %v, %v", t.ID, t.State)
		
     } else {
				log.Printf("no pending tasks")
			}
	}
