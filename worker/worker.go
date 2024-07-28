package worker
import (
    "github.com/google/uuid"
    "github.com/golang-collections/collections/queue"
    "minkube/task"
    "fmt"
)
type Worker struct {
        Name      string
        Queue     queue.Queue
        Db        map[uuid.UUID]task.Task //stores tasks, using UUIDs as keys and task.Task objects as values
        TaskCount int
}

func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
}

func (w *Worker) RunTask() {
	fmt.Println("Start or stop task")
}
func (w *Worker) StartTask() {
	fmt.Println("I will start a task")
 }
 func (w *Worker) StopTask() {
	fmt.Println("I will stop a task")
 }
 