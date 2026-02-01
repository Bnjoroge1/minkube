package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"minkube/task"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ErrResponse struct definition
type ErrResponse struct {
	HTTPStatusCode int    `json:"httpStatusCode"`
	Message        string `json:"message"`
}

// handlers for different routers
func (a *Api) StartTaskHandler(w http.ResponseWriter, r *http.Request) {
	
	if r.ContentLength == 0 {
		http.Error(w, "Request body is empty", http.StatusNoContent)
		return 
	}
	

	log.Printf("starting task handler")
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	te := task.TaskEvent{}
	err := d.Decode(&te)
	if err != nil {
		msg := fmt.Sprintf("error parsing JSON: %v", err)
		log.Printf(msg)
		w.WriteHeader(http.StatusBadRequest)
		e := ErrResponse{
			HTTPStatusCode: 400,
			Message:        msg,
		}
		json.NewEncoder(w).Encode(e)
		return
	}
	log.Printf("task event: %v", te)
	te.Task.ID = uuid.New()
	
	now := time.Now().UTC()
	te.Timestamp = now
	te.Task.StartTime = now
	te.Task.State = task.Pending

	a.Worker.AddTask(&te.Task)
	log.Printf("added task: %v", te)
	w.WriteHeader(http.StatusCreated) //specifically 201 instead of 200 because we are creating a resource thus want to be specific that this successful operation created a resource.
	json.NewEncoder(w).Encode(te)
}

// Handler for getting tasks
func (a *Api) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	//set the header to json
	a.Worker.mu.Lock()
	defer a.Worker.mu.Unlock()
	//parameters for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := 1
	limit := 100
	//parse pageStr
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	//parse limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr);err == nil && l > 0 && limit <= 1000 {
			limit = l
		}
	}


	tasks := make([]*task.Task, 0, len(a.Worker.TaskIds))
	for _, task := range a.Worker.TaskIds {
		tasks = append(tasks, task)
	}
	//get the pagination
	totalTasks := len(tasks)
	totalPages := (totalTasks + limit - 1) / limit

	//start and end indices
	start := (page - 1) * limit 
	end := start + limit

	if start  >= totalTasks {
		start = totalTasks
		end = totalTasks
	} else if end > totalTasks {
		end = totalTasks
	}

	//get the slice for this page
	var pagedTasks[]*task.Task
	if start < end {
		pagedTasks = tasks[start:end]
	} else {
		pagedTasks = []*task.Task{} //empty slice
	}

	response := map[string]interface{} {
		"tasks":       pagedTasks,
		"pagination": map[string]interface{}{
            "page":        page,
            "limit":       limit,
            "total_tasks": totalTasks,
            "total_pages": totalPages,
            "has_next":    page < totalPages,
            "has_prev":    page > 1,
        },
	}
	
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}

func (a *Api) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		//if no taskID is passed in the request, return a 400 error
		log.Printf("No taskID passed in request.\n")
		w.WriteHeader(400)
	}
	a.Worker.mu.Lock()
	defer a.Worker.mu.Unlock()
	tId, err := uuid.Parse(taskID)
	if err != nil {
		log.Printf("Error parsing taskID: %v", err)
		w.WriteHeader(400)
		return
	}
	taskToStop, ok := a.Worker.TaskIds[tId]
	if !ok {
		log.Printf("Task with ID %s not found.\n", taskID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	taskToStop.State = task.Completed
	
	a.Worker.AddTask(taskToStop)
	log.Printf("task stopped: %v", taskToStop)
	w.WriteHeader(http.StatusAccepted) //successfully stopped the task.

}

func (a *Api) GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	a.Worker.mu.Lock()
	defer a.Worker.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(a.Worker.Stats)
}

func (a *Api) HealthHandler(w http.ResponseWriter, r *http.Request){
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "up"})
}