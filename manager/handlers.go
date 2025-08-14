package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
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
	te := task.TaskEvent{}
	err = json.Unmarshal(body, &te)
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
	te.Task.StartTime = time.Now()
	a.Manager.AddTask(&te.Task)
	log.Printf("Added task %v\n", te.Task.ID)
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(te.Task)
}

func (a *Api) GetTasksHandler(w http.ResponseWriter, r *http.Request) {
	//set the header to json
	a.Manager.mu.Lock()
	defer a.Manager.mu.Unlock()

	tasks := make([]*task.Task, 0, len(a.Manager.TaskDb))
	for _, task := range a.Manager.TaskDb {
		tasks = append(tasks, task)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(a.Manager.GetTasks())
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