package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"minkube/task"
	"minkube/worker"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Manager struct {
	PendingTasks        queue.Queue
	TaskDb         map[uuid.UUID]*task.Task
	SortedTasks    []*task.Task
	EventDb        map[uuid.UUID]*task.TaskEvent
	Workers        []string
	WorkersTaskMap map[string][]uuid.UUID //maps worker to task itself
	TaskWorkerMap  map[uuid.UUID]string
	lastWorker     int //track last worker to send task to
	LastCleanupTime time.Time
	mu         sync.RWMutex 
	
}
type PaginationMetadata struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalTasks int `json:"total_tasks"`
	TotalPages int `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
} 
const MANAGER_API_KEY = "mk_manager_internal"

type PaginatedTaskResponse struct {
    Tasks      []*task.Task `json:"tasks"`
    PaginationMetadata PaginationMetadata `json:"pagination_metadata"`
}
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
	MaxIdleConns: 100,  //max number of idle connections across all hosts
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout: 90*time.Second,
	DisableKeepAlives: false,  //enable conenction reuse. 
	ForceAttemptHTTP2: true,  //use HTTP2 if available
	// Add connection tracing
     DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            log.Printf("Creating NEW connection to %s", addr)
            dialer := &net.Dialer{Timeout: 5 * time.Second}
            return dialer.DialContext(ctx, network, addr)
        },
    },
}


func New(workers []string) *Manager {
	taskDB := make(map[uuid.UUID]*task.Task)
	eventDB := make(map[uuid.UUID]*task.TaskEvent)
	workersTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	for _,worker := range workers {
		workersTaskMap[worker] = []uuid.UUID{}
	}
	manager :=&Manager{
		PendingTasks: *queue.New(),
		TaskDb: taskDB,
		EventDb: eventDB,
		Workers: workers,
		WorkersTaskMap: workersTaskMap,
		TaskWorkerMap: taskWorkerMap,
		LastCleanupTime: time.Now(),
	}
	manager.StartBackgroundCleanup()
	return manager
	}

// Get task from db
func (m *Manager) GetTask(id uuid.UUID) (*task.Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.TaskDb[id]
	if !exists {
		log.Printf("Task does not exist.")
		return nil, false
	}
	return task, true
}

//update task in db

func (m *Manager)  AddTask(t *task.Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	//adds task to manager
	m.PendingTasks.Enqueue(t)
	m.TaskDb[t.ID] = t
	if m.SortedTasks == nil{
		m.SortedTasks = make([]*task.Task, 0)
	}

	existingIndex := m.findTaskInSortedList(t.ID)
	if existingIndex >= 0 {
		m.SortedTasks[existingIndex] = t
		log.Printf("Updated existing task %s in SortedTasks at index %d", t.ID, existingIndex)
	} else {
		m.SortedTasks = m.InsertSorted(m.SortedTasks, t)
		log.Printf("Inserted new task %s into Sorted Tasks, total count: %d", t.ID, len(m.SortedTasks))
	}
	log.Printf("Added task %v to pending queue. Queue size: %d", t.ID, m.PendingTasks.Len())
}
func (m *Manager) findTaskInSortedList(taskID uuid.UUID) int {
    for i, task := range m.SortedTasks {
        if task != nil && task.ID == taskID {
            return i
        }
    }
    return -1
}

func (m *Manager) InsertSorted(tasks []*task.Task, newTask *task.Task)[]*task.Task{
	index := sort.Search(len(tasks), func(i int)bool {
		return tasks[i].StartTime.After(newTask.StartTime)
	})
	tasks = append(tasks, nil)
	copy(tasks[index+1:], tasks[index:])
	tasks[index] = newTask

	return tasks
}
const (
	CLEANUP_INTERVAL      = 1 * time.Hour
	TASK_RETENTION_HOURS = 24
)
func(m *Manager) StartBackgroundCleanup(){
	go func ()  {
		ticker := time.NewTicker(CLEANUP_INTERVAL)
		for range ticker.C{
			log.Printf("Starting background cleanuo")
			//m.CleanUpTasks()
			log.Printf("Background clean up completed")
		}
	}()
}
func (m *Manager) CleanUpTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-TASK_RETENTION_HOURS * time.Hour)
	log.Printf("Cleaning up tasks completed before %v", cutoff)

	for id, t := range m.TaskDb {
		if t.State == task.Completed && t.StartTime.Before(cutoff) {
			log.Printf("Removing completed task %s from db", t.ID)
			// remove from TaskWorkerMap
			if worker, ok := m.TaskWorkerMap[id]; ok {
				// remove from WorkersTaskMap
				taskIDs := m.WorkersTaskMap[worker]
				for i, taskID := range taskIDs {
					if taskID == id {
						m.WorkersTaskMap[worker] = append(taskIDs[:i], taskIDs[i+1:]...)
						break
					}
				}
				delete(m.TaskWorkerMap, id)
			}
			// remove from EventDb
			delete(m.EventDb, id)
			// remove from TaskDb
			delete(m.TaskDb, id)
		}
	}
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
func (m *Manager) UpdateTasks() {
    for {
        log.Println("Checking for task updates from workers")
        m.updateTasks()
        log.Println("Task updates completed")
        log.Println("Sleeping for 15 seconds")
        time.Sleep(15 * time.Second)
    }
}
func (m *Manager) ProcessTasks() {
    for {
        log.Println("Processing any tasks in the queue")
        m.SendWork()
        log.Println("Sleeping for 10 seconds")
        time.Sleep(10 * time.Second)
    }
}
// updates the status of tasks
func (m *Manager) updateTasks() error {
	//get the status of tasks in manager's queue from workers and update it.
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	m.mu.RLock() //read-heavy lock
	workers := make([]string, len(m.Workers))
	copy(workers, m.Workers)
	m.mu.RUnlock()

	log.Printf("Manager state: TaskDb=%d tasks, PendingTasks=%d", len(m.TaskDb),m.PendingTasks.Len())


	var wg sync.WaitGroup
	errCh := make(chan error, len(workers))

	for _, w := range workers {
		wg.Add(1) //increment wg counter by one.
		work := w
		go func() {
			defer wg.Done()

			

			url := fmt.Sprintf("http://%s/tasks?page=1&limit=1000", work)

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				log.Printf("Error retrieving tasks for this worker %s: %v", work, err)
				errCh <- fmt.Errorf("error retrieving tasks for worker %s: %w", work, err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+MANAGER_API_KEY)

			
			resp, err := httpClient.Do(req)
			if err != nil {
				log.Printf("Error receiving tasks for worker %s:%v\n",work,  err)
				return
			}
			defer resp.Body.Close()

			decoder := json.NewDecoder(resp.Body)
			if resp.StatusCode != http.StatusOK {
				log.Printf("Error: retrieved list from worker: %s. Received status code: %d", work, resp.StatusCode)
				//create the error response
				resp_err := worker.ErrResponse{}
				dec_err := decoder.Decode(&resp_err)
				if dec_err != nil {
					log.Printf("Error decoding the error response: %s", resp_err.Message)
				}
				errCh <- fmt.Errorf("error response from worker %s: status %d", work, resp.StatusCode)
				return
			}
			var paginatedResp PaginatedTaskResponse
			recv_err := decoder.Decode(&paginatedResp)
			if recv_err != nil {
				log.Printf("Could not decode paginated response of tasks from %s: %v\n", work, recv_err)
				errCh <- fmt.Errorf("could not decode list of tasks from %s: %w", work, recv_err)
				return
			}
			log.Printf("Received %d tasks from worker %s", len(paginatedResp.Tasks), work)


			m.mu.Lock()
			updatedCount := 0
			orphanedCount := 0
			for i, workerTask := range paginatedResp.Tasks {
				if i < 3 {
					log.Printf("Task %d: ID=%s, State-%v, Name=%s",i,  workerTask.ID, workerTask.State, workerTask.Name)
				}
				// The logic from UpdateTaskState is now here, inside the lock.
				log.Printf("Manager task State: %v, WorkerTask State: %v", m.TaskDb[workerTask.ID], workerTask.State)
				if managerTask, ok := m.TaskDb[workerTask.ID]; ok {
					if managerTask.State != workerTask.State {
						log.Printf("Updating task %s state from %v to %v",
							managerTask.ID, managerTask.State, workerTask.State)
						managerTask.State = workerTask.State
						managerTask.ContainerID = workerTask.ContainerID
						managerTask.StartTime = workerTask.StartTime
						managerTask.EndTime = workerTask.EndTime
					}
				}else{
					log.Printf("Task %s exists on worker %s but not in manager TaskDB", workerTask.ID, work)
					log.Printf("Worker task state: %v, Manager TaskDb size: %d", workerTask.State, len(m.TaskDb))
					 
                    log.Printf("ORPHANED TASK: %s exists on worker %s but not in manager TaskDb", 
                        workerTask.ID, work)
                    
                    // Create a copy of the worker task and add to manager state
                    taskCopy := *workerTask
                    m.TaskDb[workerTask.ID] = &taskCopy
                    m.TaskWorkerMap[workerTask.ID] = work

				if m.SortedTasks == nil{
					m.SortedTasks = make([]*task.Task, 0)
				}
				m.SortedTasks = m.InsertSorted(m.SortedTasks, &taskCopy)
				log.Printf("Added orphaned task %s to Sorted Task list", &taskCopy.ID)
                    
                    // Add to worker's task list if not already there
                    found := false
                    for _, existingID := range m.WorkersTaskMap[work] {
                        if existingID == workerTask.ID {
                            found = true
                            break
                        }
                    }
                    if !found {
                        m.WorkersTaskMap[work] = append(m.WorkersTaskMap[work], workerTask.ID)
                    }
                    
                    orphanedCount++
                    log.Printf("Added orphaned task %s to manager state", workerTask.ID)
                }

				}
		
			m.mu.Unlock()
			if updatedCount > 0 || orphanedCount > 0 {
                log.Printf("Worker %s: updated %d tasks, recovered %d orphaned tasks", 
                    work, updatedCount, orphanedCount)
            }
		}()
	}
	wg.Wait()
	close(errCh)

	// Check for any errors sent by the goroutines.
	for err := range errCh {
		if err != nil {
			// Return the first error encountered.
			return err
		}
	}

	log.Println("Finished task status update cycle.")
	return nil
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
		log.Printf("Task %s is not a *task  instance", e)
		return
	}

	log.Printf("Pulled %v task off pending queue", t)
	//if there's taks that are still pending. 
	//select worker to run task
	w, error  := m.SelectWorker()
	if error != nil {
		log.Printf("No worker selected %s", error)
		return
	}

	t.State = task.Scheduled
	//create a task event

	// The TaskEvent is created as before.
	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     t.State,
		Timestamp: time.Now().UTC(),
		Task:      *t,
	}

	// A derived context for the HTTP request can be created from the parent.
	// This is useful if you want a different timeout or cancellation for the request
	// without affecting the parent context. For this example, we'll use the parent directly,
	// but this is how you would create a derived one:
	// httpCtx, httpCancel := context.WithTimeout(parentCtx, 25*time.Second)
	//

	data, err := json.Marshal(&te)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.", t)
		m.AddTask(t)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s/tasks", w)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		m.mu.Lock()
		m.AddTask(t)
		m.mu.Unlock()
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Error connecting to %v: %v", w, err)
		m.mu.Lock()
		m.AddTask(t)
		m.mu.Unlock()
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

	log.Printf("Successfully sent task %v to worker %s", t.ID, w)
	m.EventDb[t.ID] = &te //matching task to event db
	m.WorkersTaskMap[w] = append(m.WorkersTaskMap[w], t.ID)
	m.TaskWorkerMap[t.ID] = w
	m.TaskDb[t.ID] = t
	log.Printf("Successfully added task %v to eventDB, and the task maps.", t.ID)
	
}

func (m *Manager) GetTasks() []*task.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
    tasks := []*task.Task{}
    for _, t := range m.TaskDb {
        tasks = append(tasks, t)
    }
    return tasks
}