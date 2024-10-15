package worker

import (
	"fmt"
	"log"
	"minkube/task"
	"strings"
	"sync"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)
type Worker struct  {
        Name      string
        Queue     queue.Queue
        TaskIds   map[uuid.UUID]*task.Task   //stores the task ids which can be referenced in the manager by the ID
        TaskCount int
        Stats *Stats
        mu sync.Mutex
}
func (w *Worker)  AddTask(t task.Task) {
    w.Queue.Enqueue(t)
}

func (w *Worker) CollectStats() {
	for  {
        log.Println("Collecting stats")
        w.Stats = GetStats()
        w.TaskCount = w.Stats.TaskCount 
        time.Sleep(15 * time.Second)
    }
}

func (w *Worker) RunTask()  (task.DockerResult, *task.Task){
    //responsible for identifying the task’s current state, and then either starting or stopping a task based on the state.

    /* There are two possible scenarios for handling tasks:
        a) a task is being submitted for the first time, so the Worker will not know about it

        b) a task is being submitted for the Nth time, where the task submitted represents the desired state to which the current task should transition.

        The worker will interpret tasks it already has in its Db field as existing tasks, that is tasks that it has already seen at least once. If a task is in the Queue but not the Db, then this is the first time the worker is seeing the task, and we default to starting it.       
    */
        //retrieve task from queue if it exists.
        log.Printf("Running task")
        
        t := w.Queue.Dequeue()
        if t == nil {
            log.Println("No tasks in the queue")
            return task.DockerResult{Error: nil}, nil
        }
        //cast to task.Task type
        taskQueued := t.(task.Task)
        log.Printf("Task yet to run %v is in state %v\n", taskQueued.ID, taskQueued.State)
        // Check if the task is already in a terminal state
    if taskQueued.State == task.Completed || taskQueued.State == task.Failed {
        log.Printf("Task %s is already in terminal state %d. Skipping.", taskQueued.ID, taskQueued.State)
        return task.DockerResult{}, &taskQueued
    }

        
        //Check if task is already in database
        taskPersisted := w.TaskIds[taskQueued.ID]
        
        if taskPersisted == nil {
            taskPersisted = &taskQueued
            w.TaskIds[taskQueued.ID] = &taskQueued
        }
        
        //validate transition state is correct. 
        var result task.DockerResult
        var updatedTask *task.Task

        if taskPersisted.State == task.Pending {
            taskPersisted.State = task.Scheduled
        }
        log.Printf("Task %v is in state %v\n", taskQueued.ID, taskQueued.State)
        
        if task.ValidateStateTransition(taskPersisted.State, taskQueued.State) {
            log.Printf("Valid transition from %v to %v\n", taskPersisted.State, taskQueued.State)
            w.mu.Lock()
            defer w.mu.Unlock()
            switch taskQueued.State {
                
            case task.Scheduled:
                //if it's scheduled, we want to start the task.
                log.Printf("Starting scheduled task %v\n", taskQueued.ID)
                result, updatedTask  = w.StartTask(taskQueued)
                log.Printf("Task that was scheduled %v is now in state %v\n", updatedTask.ID, updatedTask.State)
                if result.Error != nil {
                    log.Printf("Error starting task %v:%v \n", taskQueued.ID, result)
                    updatedTask.State = task.Failed
                }
                return result, updatedTask
            case task.Running:
                
                log.Printf("task is still running")
                return task.DockerResult{}, &taskQueued
            case task.Completed:
                //if the task's state from the queue is completed we want to stop the task(and therefore transition it to Completed. ) 
                result = w.StopTask(taskQueued)
                updatedTask = &taskQueued
                if result.Error != nil {
                    log.Printf("Error  in the stop fucntion of task %v:%v \n", taskQueued.ID, result)
                }
                
                return result, updatedTask
            
            default:
                result.Error = fmt.Errorf("unexpected task state: %v", taskQueued.State)
                log.Printf("Unexpected task state for task %v: %v\n", taskQueued.ID, taskQueued.State)
            }
            
        } else {
            err := fmt.Errorf("you cant transition. Invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
            result.Error = err
            return result, nil
        }
        return result, updatedTask
    }


func (w *Worker) StartTask(t task.Task) (task.DockerResult, *task.Task){
    //starts a given task.
    log.Printf("StartTask: Beginning for task %v", t.ID)

 
    defer func() {
        if r := recover(); r != nil {
            log.Printf("StartTask: Recovered from panic in task %v: %v", t.ID, r)
        }
    }()
    //set up necessary fields
    t.StartTime = time.Now().UTC()

    config := task.NewConfig(&t)
    d := task.NewDocker(config)
    if d == nil {
        log.Printf("StartTask: Failed to create Docker object for task %v", t.ID)
        t.State = task.Failed
        w.TaskIds[t.ID] = &t
        return task.DockerResult{}, nil
    }

    //run the task
    result := d.Run()
    
    if result.Error != nil {
        log.Printf("Error starting task %v:%v \n", t.ID, result)
        t.State = task.Failed
        w.TaskIds[t.ID] = &t
        return result, nil
    }
    log.Printf("Docker container started successfully for task %v", t.ID)

    t.ContainerID = result.ContainerId
    t.State = task.Scheduled
    w.TaskIds[t.ID] = &t

    log.Printf("Started task %v with container ID %v\n", t.ID, t.ContainerID)
    return result, w.TaskIds[t.ID]

}
 func (w *Worker) StopTask(t task.Task) task.DockerResult {
    w.mu.Lock()
    defer w.mu.Unlock()
    log.Printf("Attempting to stop container with ID: %s", t.ContainerID)
    //receives a task and stops it
    config := task.NewConfig(&t)
    d := task.NewDocker(config)

    result := d.Stop(t.ContainerID)
    if result.Error != nil {
        log.Printf("Error occurred in worker stopping container %v: %v \n", t.ContainerID, result)
    }
    t.EndTime = time.Now().UTC()
    t.State = task.Completed
    w.TaskIds[t.ID] = &t 
    log.Printf("Stopped and removed container %v for task %v \n", t.ContainerID, result)
    return result 
 }

 //funciton to get all tasks
 func (w *Worker) GetTasks() []task.Task {
    tasks := make([]task.Task, 0, len(w.TaskIds))
    for _, t := range w.TaskIds {
        tasks = append(tasks, *t)
    }
    return tasks
}
 //function to check if task is still running
 func (w *Worker) IsTaskRunning(t task.Task, containerID string) (bool, error) {
    if containerID == "" {
        return false, nil
    }
    config := task.NewConfig(&t)
    d := task.NewDocker(config)
    isRunning, err := d.IsRunning(containerID)
    if err != nil {
        // If the container is not found, it's not running
        if strings.Contains(err.Error(), "No such container") {
            return false, nil
        }
        return false, fmt.Errorf("error checking container status: %v", err)
    }
    return isRunning, nil
}
 func (w *Worker) MonitorTasks() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            w.mu.Lock()
            for id, t := range w.TaskIds {
                if t.State != task.Completed && t.State != task.Failed {
                    isRunning, err := w.IsTaskRunning(*t, t.ContainerID)
                    log.Printf("MonitorTasks: Is task %v running? %v", id, isRunning)
                    log.Printf("MonitorTasks: Checking task %v (State: %v, ContainerID: %v)", id, t.State, t.ContainerID)
                    if err != nil {
                        log.Printf("Error checking task status for %s: %v", id, err)
                        continue
                    }
                    if isRunning {
                        if t.State != task.Running {
                            log.Printf("Task %s is now running. Updating state to Running", id)
                            t.State = task.Running
                        }
                    } else {
                        if t.State == task.Running {
                            log.Printf("Task %s is no longer running. Updating state to Completed", id)
                            t.State = task.Completed
                            t.EndTime = time.Now().UTC()
                            log.Printf("Task %s completed. Start time: %v, End time: %v", id, t.StartTime, t.EndTime)
                        } else if t.State == task.Scheduled {
                            log.Printf("Task %s failed to start. Updating state to Failed", id)
                            t.State = task.Failed
                            t.EndTime = time.Now().UTC()
                            log.Printf("Task %s failed. Start time: %v, End time: %v", id, t.StartTime, t.EndTime)
                        }
                    }
                }
            }
            w.mu.Unlock()
        }
    }
}