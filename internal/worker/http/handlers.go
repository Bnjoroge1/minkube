package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"minkube/internal/task"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ErrResponse struct definition
type ErrResponse struct {
	HTTPStatusCode int    `json:"httpStatusCode"`
	Message        string `json:"message"`
}

func writeSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	e := ErrResponse{
		HTTPStatusCode: statusCode,
		Message:        message,
	}
	json.NewEncoder(w).Encode(e)
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
		writeErrorResponse(w, http.StatusBadRequest, msg)
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
	writeSuccessResponse(w, http.StatusCreated, te)//specifically 201 instead of 200 because we are creating a resource thus want to be specific that this successful operation created a resource.
}
func (a *Api) GetDockerTaskLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskId := chi.URLParam(r, "taskID")
	if taskId == "" {
		msg := "Task ID is empty. Please provide one \n"
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	tid, err := uuid.Parse(taskId)
	if err != nil {
		msg := "Task ID is not a valid UUID \n"
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	t, ok := a.Worker.GetTask(tid)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "task not found on this worker")
		return
	}
	if t.ContainerID == "" {
		writeErrorResponse(w, http.StatusConflict, "task has no container yet (not running)")
		return
	}
	logResponse := a.Worker.GetDockerTaskLogs(t)
	if logResponse.Error != nil {
		log.Printf("GetDockerTaskLogsHandler: GetDockerTaskLogs error: %v", logResponse.Error)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to get logs: %v", logResponse.Error))
		return
	}
	f, ok := w.(http.Flusher)
	if !ok {
		writeErrorResponse(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)

	errp := a.Worker.ProcessDockerStream(logResponse, func(logFormat task.DockerLogFormat) error {
		logEntry, err := json.Marshal(logFormat)
		if err != nil {
			return err
		}
		w.Write(logEntry)
		w.Write([]byte("\n"))
		f.Flush()
		return nil
	})
	if errp != nil {
		log.Printf("GetDockerTaskLogsHandler: stream error: %v", errp)
		return
	}
}

// Handler for getting tasks
func (a *Api) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	//set the header to json
	// 	//parameters for pagination
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

	a.Worker.RLock()
	tasks := make([]*task.Task, 0, len(a.Worker.TaskIds))
	for _, t := range a.Worker.TaskIds {
		taskCopy := *t
		tasks = append(tasks, &taskCopy)
	}
	a.Worker.RUnlock()
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
	
	
	writeSuccessResponse(w, http.StatusOK, response)
}

func (a *Api) StopTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		//if no taskID is passed in the request, return a 400 error
		msg := fmt.Sprintf("task id is not passed in request %s", taskID)
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	a.Worker.Lock()
	tId, err := uuid.Parse(taskID)
	if err != nil {
		a.Worker.Unlock()
		msg := fmt.Sprintf("COuld not parse id %s", taskID)
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	taskToStop, ok := a.Worker.TaskIds[tId]
	if !ok {
		a.Worker.Unlock()
		msg := fmt.Sprintf("Task with ID %s not found.\n", taskID)
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	taskToStop.State = task.Completed
	
	a.Worker.AddTask(taskToStop)
	log.Printf("task stopped: %v", taskToStop)
	a.Worker.Unlock()
	writeSuccessResponse(w, http.StatusOK, taskToStop)

}

func (a *Api) GetStatsHandler(w http.ResponseWriter, r *http.Request) {
	a.Worker.RLock()
	//create a copy of stats
	if a.Worker.Stats == nil{
		a.Worker.RUnlock()
		msg := "No stats to show"
		writeErrorResponse(w, http.StatusBadRequest, msg)
		return
	}
	statsCopy := *a.Worker.Stats
	a.Worker.RUnlock()
	writeSuccessResponse(w, http.StatusOK, statsCopy)
}

func (a *Api) HealthHandler(w http.ResponseWriter, r *http.Request){
	writeSuccessResponse(w, http.StatusOK, map[string]string{"status": "up"})
}


