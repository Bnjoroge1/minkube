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

func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
}

func (w *Worker) RunTask() {
	fmt.Println("Start or stop task")
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
 