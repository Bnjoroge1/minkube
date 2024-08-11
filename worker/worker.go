package worker
import (
    "github.com/google/uuid"
    "github.com/golang-collections/collections/queue"
    "minkube/task"
    "fmt"
    "time"
    "log"
    
)
type Worker struct  {
        Name      string
        Queue     queue.Queue
        TaskIds   map[uuid.UUID]*task.Task   //stores the task ids which can be referenced in the manager by the ID
        TaskCount int
}
func (w *Worker)  AddTasK(t task.Task) {
    w.Queue.Enqueue(t)
}
func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
}

func (w *Worker) RunTask()  task.DockerResult{
    //responsible for identifying the task’s current state, and then either starting or stopping a task based on the state.

    /* There are two possible scenarios for handling tasks:
        a) a task is being submitted for the first time, so the Worker will not know about it

        b) a task is being submitted for the Nth time, where the task submitted represents the desired state to which the current task should transition.

        The worker will interpret tasks it already has in its Db field as existing tasks, that is tasks that it has already seen at least once. If a task is in the Queue but not the Db, then this is the first time the worker is seeing the task, and we default to starting it.       
    */
        //retrieve task from queue if it exists.
        t := w.Queue.Dequeue()
        if t == nil {
            log.Println("No tasks in the queue")
            return task.DockerResult{Error: nil}
        }
        //cast to task.Task type
        taskQueued := t.(task.Task)
        
        //Check if task is already in database
        taskPersisted := w.TaskIds[taskQueued.ID]
        if taskPersisted == nil {
            taskPersisted = &taskQueued
            w.TaskIds[taskQueued.ID] = &taskQueued
        }
        
        //validate transition state is correct. 
        var result task.DockerResult
        if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
            switch taskQueued.State {
            case task.Scheduled:
                //if it's scheduled, we want to start the task.
                result = w.StartTask(taskQueued)
            case task.Completed:
                //if the task's state from the queue is completed we want to stop the task(and therefore transition it to Completed. ) 
                result = w.StopTask(taskQueued)
            default:
                result.Error = errors.New("We should not get here")
            }
        } else {
            err := fmt.Errorf("You cant transition. Invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
            result.Error = err
            return result
        }
        return result
    }
}
func (w *Worker) StartTask(t task.Task) task.DockerResult{
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
        return result
    }
    t.State = task.Running
    t.ContainerID = result.ContainerId
    w.TaskIds[t.ID] = &t
    return result

}
 func (w *Worker) StopTask(t task.Task) task.DockerResult {
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
 