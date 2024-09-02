package worker
import (
    "github.com/google/uuid"
    "github.com/golang-collections/collections/queue"
    "minkube/task"
    "fmt"
    "time"
    "log"
    "errors"
    
)
type Worker struct  {
        Name      string
        Queue     queue.Queue
        TaskIds   map[uuid.UUID]*task.Task   //stores the task ids which can be referenced in the manager by the ID
        TaskCount int
}
func (w *Worker)  AddTask(t task.Task) {
    w.Queue.Enqueue(t)
}
func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
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
        log.Printf("Task %v is in state %v\n", taskQueued.ID, taskQueued.State)
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
        if task.ValidateStateTransition(taskPersisted.State, taskQueued.State) {
            switch taskQueued.State {
                
            case task.Scheduled:
                //if it's scheduled, we want to start the task.
                result, updatedTask  = w.StartTask(taskQueued)
                if result.Error != nil {
                    return result, updatedTask
                }
                log.Printf("Task %v is now in state %v\n", taskQueued.ID, taskQueued.State)
                return result, updatedTask
            case task.Running:
                // The task is already running, so we don't need to do anything
                log.Printf("Task %v is already running\n", taskQueued.ID)
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
                result.Error = fmt.Errorf("Unexpected task state: %v", taskQueued.State)
                log.Printf("Unexpected task state for task %v: %v\n", taskQueued.ID, taskQueued.State)
            }
            }
        } else {
            err := fmt.Errorf("You cant transition. Invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
            result.Error = err
            return result, nil
        }
        return result, updatedTask
    }


func (w *Worker) StartTask(t task.Task) (task.DockerResult, *task.Task){
    //starts a given task.

    //set up necessary fields
    t.StartTime = time.Now().UTC()
    config := task.NewConfig(&t)
    d := task.NewDocker(config)

    //run the task
    result := d.Run()
    if result.Error != nil {
        log.Printf("Error starting task %v:%v \n", t.ID, result)
        t.State = task.Failed
        w.TaskIds[t.ID] = &t
        return result, nil
    }
    t.ContainerID = result.ContainerId
    t.State = task.Running
    w.TaskIds[t.ID] = &t

    log.Printf("Started task %v with container ID %v\n", t.ID, t.ContainerID)
    return result, w.TaskIds[t.ID]

}
 func (w *Worker) StopTask(t task.Task) task.DockerResult {
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
 