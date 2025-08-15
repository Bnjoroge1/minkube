package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"minkube/task"

	"github.com/docker/go-connections/nat"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ErrResponse struct definition
type ErrResponse struct {
	HTTPStatusCode int    `json:"httpStatusCode"`
	Message        string `json:"message"`
}


var bytesPool = sync.Pool{
		New: func() interface{}{
		return make([]byte, 0, 1024)
	},
} //create a pool of decoders to be reused to avoid repeated allocation of reflection metadata

func (a *Api) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024) //adding 10KB limit for max bytes per request

	buf := bytesPool.Get().([]byte)
	defer func () {
		buf = buf[:0]
		bytesPool.Put(buf)
	}()


	body, err := io.ReadAll(r.Body)
                               
    if err != nil{                                                       
        msg := fmt.Sprintf("Error unmarshalling body: %v\n", err)
        log.Printf(msg)
        w.WriteHeader(400)
        e := ErrResponse{
            HTTPStatusCode: 400,
            Message:        msg,
        }
        json.NewEncoder(w).Encode(e)
        return
    }
     var taskRequestEvent task.StartTaskEventRequest 
	err = json.Unmarshal(body, &taskRequestEvent)
	if err != nil {
		msg := fmt.Sprintf("Error unmarshalling body: %v\n", err)
		log.Printf(msg)
		w.WriteHeader(400)
		e := ErrResponse{
			HTTPStatusCode : 400,
			Message: msg,
		}
		json.NewEncoder(w).Encode(e)
		return 
	}
	if taskRequestEvent.Task.Name == "" {
        msg := "Task name is required"
        log.Printf(msg)
        w.WriteHeader(http.StatusBadRequest)
        e := ErrResponse{
            HTTPStatusCode: http.StatusBadRequest,
            Message:        msg,
        }
        json.NewEncoder(w).Encode(e)
        return
    }
    if taskRequestEvent.Task.Image == "" {
        msg := "Task image is required"
        log.Printf(msg)
        w.WriteHeader(http.StatusBadRequest)
        e := ErrResponse{
            HTTPStatusCode: http.StatusBadRequest,
            Message:        msg,
        }
        json.NewEncoder(w).Encode(e)
        return
    }
    // Validate memory limits - Docker requires minimum 6MB (6 * 1024 * 1024 bytes)
    const minMemoryMB = 6
    const minMemoryBytes = minMemoryMB * 1024 * 1024
    // Assume memory is specified in MB in the request
    memoryInBytes := taskRequestEvent.Task.Memory * 1024 * 1024
    if memoryInBytes < minMemoryBytes {
        msg := fmt.Sprintf("Memory must be at least %dMB, got %dMB", minMemoryMB, taskRequestEvent.Task.Memory)
        log.Printf(msg)
        w.WriteHeader(http.StatusBadRequest)
        e := ErrResponse{
            HTTPStatusCode: http.StatusBadRequest,
            Message:        msg,
        }
        json.NewEncoder(w).Encode(e)
        return
    }
    // Validate disk space if specified
    if taskRequestEvent.Task.Disk > 0 && taskRequestEvent.Task.Disk < 1 {
        msg := "Disk space must be at least 1MB if specified"
        log.Printf(msg)
        w.WriteHeader(http.StatusBadRequest)
        e := ErrResponse{
            HTTPStatusCode: http.StatusBadRequest,
            Message:        msg,
        }
        json.NewEncoder(w).Encode(e)
        return
    }

    
	//create complete task with server-generated fields
	fullTask := task.Task{
		ID: uuid.New(),
		Name: taskRequestEvent.Task.Name,
		Image: taskRequestEvent.Task.Image,
		Memory: memoryInBytes,
		Disk: taskRequestEvent.Task.Disk,
		RestartPolicy: taskRequestEvent.Task.RestartPolicy,
		PortBindings: taskRequestEvent.Task.PortBindings,
		State: task.Pending,
		StartTime: time.Now(),
		ExposedPortSet: make(nat.PortSet),
		HostPorts: make(nat.PortMap),
	}
	te := task.TaskEvent{
		ID: fullTask.ID,
		State: fullTask.State,
		Timestamp: fullTask.StartTime,
		Task: fullTask, 
	}

	a.Manager.AddTask(&te.Task)
	log.Printf("Added task %v\n", te.Task.ID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(te)
}
func parsePaginationParams(r *http.Request) (page, limit int, err error) {
    pageStr := r.URL.Query().Get("page")
    if pageStr == "" {
        page = 1
    } else {
        page, err = strconv.Atoi(pageStr)
        if err != nil || page < 1 {
            return 0, 0, fmt.Errorf("invalid page parameter")
        }
    }

    limitStr := r.URL.Query().Get("limit")
    if limitStr == "" {
        limit = 100 // Default limit following Minkube patterns
    } else {
        limit, err = strconv.Atoi(limitStr)
        if err != nil || limit < 1 || limit > 1000 {
            return 0, 0, fmt.Errorf("limit must be between 1 and 1000")
        }
    }

    return page, limit, nil
}
func (a *Api) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Get TaskHandler: Starting request processing")
	page, limit, err := parsePaginationParams(r)
	if err != nil {
        log.Printf("GetTasksHandler: Pagination error: %v", err)
        w.WriteHeader(http.StatusBadRequest)
        e := ErrResponse{
            HTTPStatusCode: http.StatusBadRequest,
            Message:        fmt.Sprintf("Invalid pagination parameters: %v", err),
        }
        json.NewEncoder(w).Encode(e)
        return
    }

	//set the header to json
	a.Manager.mu.RLock()
	taskCount := len(a.Manager.TaskDb)
	tasks := make([]*task.Task, 0, min(limit, taskCount))
	// Collect tasks with pagination
     start := (page - 1) * limit
     current := 0
	

	
	for _, task := range a.Manager.TaskDb {
		if current >= start && len(tasks) < limit {
            // Create copy to avoid race conditions
            taskCopy := *task
            tasks = append(tasks, &taskCopy)
        }
	
		current++
		if len(tasks) >= limit {
			break
		}
	}
	// Release lock immediately after data collection
	a.Manager.mu.RUnlock()
    
    log.Printf("GetTasksHandler: Collected %d tasks from %d total", len(tasks), taskCount)

	// Calculate pagination metadata
	totalPages := (taskCount + limit - 1) / limit
	pagination := PaginationMetadata{
		Page:       page,
		Limit:      limit,
		TotalTasks:      taskCount,
		TotalPages: totalPages,
	}

	response := PaginatedTaskResponse{
		Tasks:      tasks,
		PaginationMetadata: pagination,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func min(a, b int) int {
	if a < b{
		return a
	}
	return b
}
func (a *Api) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		//if no taskID is passed in the request, return a 400 error
		log.Printf("No taskID passed in request.\n")
		w.WriteHeader(400)
	}
	a.Manager.mu.Lock()
	defer a.Manager.mu.Unlock()
	tId, err := uuid.Parse(taskID)
	if err != nil {
		log.Printf("Error parsing taskID: %v", err)
		w.WriteHeader(400)
		return
	}
	taskToStop, ok := a.Manager.TaskDb[tId]
	if !ok {
		log.Printf("Task with ID %s not found.\n", taskID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	taskToStop.State = task.Completed
	
	a.Manager.AddTask(taskToStop)
	log.Printf("task stopped: %v", taskToStop)
	w.WriteHeader(http.StatusNoContent) //successfully stopped the task.

}