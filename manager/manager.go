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
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Manager struct {
	PendingTasks        queue.Queue
	TaskDb         map[uuid.UUID]*task.Task
	EventDb        map[uuid.UUID]*task.TaskEvent
	Workers        []string
	WorkersTaskMap map[string][]uuid.UUID //maps worker to task itself
	TaskWorkerMap  map[uuid.UUID]string
	lastWorker     int //track last worker to send task to
	mu         sync.Mutex 
		

}

// Get task from db
func (m *Manager) GetTask(id uuid.UUID) (*task.Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.TaskDb[id]
	if !exists {
		log.Printf("Task does not exist.")
		return nil, false
	}
	return task, true
}

//update task in db
func (m *Manager) UpdateTaskState(id uuid.UUID, state task.State) error {

	task, exists := m.TaskDb[id]; 
	if !exists {
		return fmt.Errorf("task does not exist in database with id: %s", id)
	}
	if task.State != state {
		log.Printf("Updating task %s state from %v to %v", task.ID, task.State, state)
		task.State = state
	}
	m.TaskDb[id] = task
	return nil
	
}
func (m *Manager)  AddTask(task *task.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()

	//adds task to manager
	m.PendingTasks.Enqueue(task)
}


func (m *Manager) SelectWorker() (string, error) {
	// schedule task to workers
	// given a task, evaluate all resources available in pool of workers to find suitable worker.
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
	//get the status of tasks in manager's queue from workers and update it.

	m.mu.Lock()
	workers  := make([]string, len(m.Workers))
	copy(workers, m.Workers)
	m.mu.Unlock()

	for _, w := range workers {
		url := fmt.Sprintf("http://%s/tasks", w)

		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error retrieving tasks for this worker: %s", w)
			continue
		}
		decoder := json.NewDecoder(resp.Body)
		if resp.StatusCode != http.StatusOK {
			log.Printf("Error: retrieveed list from worker: %s. Received status code: %d",w, resp.StatusCode)
			//create the error resposne
			resp_err := worker.ErrResponse{}
			dec_err := decoder.Decode(&resp_err)
			if dec_err != nil {
				log.Printf("Error decoding the error: %s", dec_err.Error())
				
			}
			resp.Body.Close()
			continue

		}
		var recv_tasks []*task.Task
		recv_err := decoder.Decode(&recv_tasks)
		if recv_err != nil {
			log.Printf("Could not get list of tasks from %s\n", recv_err.Error())
			resp.Body.Close()
			continue
		}
		m.mu.Lock()
		for _, recv_task :=  range recv_tasks {
			ok := m.UpdateTaskState(recv_task.ID, recv_task.State)
			if ok != nil {
				log.Printf("Error updating task status for worekr: %s. Got error: %s", w, ok.Error())
			}
		}
		m.mu.Unlock()
		

	}

}

// sends tasks to workers
func (m *Manager) SendWork() {
	log.Printf("Sending tasks to workers")
	m.mu.Lock()
	if m.PendingTasks.Len() == 0{
		m.mu.Unlock()
		log.Printf("no pending tasks")
		return 
	}
	
	
	e := m.PendingTasks.Dequeue()
	m.mu.Unlock()
	t, ok := e.(*task.Task)
	if !ok {
		log.Printf("Task %s is not a task event isntance", e)
	}
	log.Printf("Pulled %v task off pending queue", t)
	//if there's taks that are still pending. 
	//select worker to run task
	w, error  := m.SelectWorker()
	if error != nil {
		log.Printf("No worker selected %s", error)
	}
	t.State = task.Scheduled
	//create a task event
	te := task.TaskEvent{
		ID: uuid.New(),
		State: t.State,
		Timestamp: time.Now().UTC(),
		Task: *t,  //send a copy of task to the task event.
	}

	data, err := json.Marshal(&t)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.", t)
	}

	url := fmt.Sprintf("http://%s/tasks", w)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error connecting to %v: %v", w, err)
		m.PendingTasks.Enqueue(t)
		return
	}
	defer resp.Body.Close()  //must always close resp body. 

	//lock manager state again
	m.mu.Lock()
	defer m.mu.Unlock()

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			fmt.Printf("Error decoding response: %s\n", e.Message)
			return
		}
		m.AddTask(t)
		log.Printf("Response error: %d", e.HTTPStatusCode)
		return
	}
	m.EventDb[t.ID] = &te //matching task to event db
	m.WorkersTaskMap[w] = append(m.WorkersTaskMap[w], t.ID)
	m.TaskWorkerMap[t.ID] = w
	m.TaskDb[t.ID] = t
	
}
