package worker

import (
	"fmt"
	"log"
	"minkube/task"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Worker struct {
	Name         string
	Queue        queue.Queue
	TaskIds      map[uuid.UUID]*task.Task //stores the task ids which can be referenced in the manager by the ID
	TaskCount    int
	Stats        *Stats
	DockerClient *client.Client
	mu           sync.RWMutex
}

func (w *Worker) AddTask(t *task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) CollectStats() {
	for {
		log.Println("Collecting stats")
		w.Stats = GetStats()
		w.TaskCount = w.Stats.TaskCount
		time.Sleep(15 * time.Second)
	}
}
func (w *Worker) RunTasks() {
	//another goroutine that loops over the queue and runs any existing tasks
	for {
		if w.Queue.Len() != 0 {

			result, t := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
				w.updateTaskState(t.ID, task.Failed)


				//decide if we need to retry depending on the error
				//
			}
			if t != nil {
				log.Printf("Task %v is with state %v\n", t.ID, t.State)
			}

		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 10 seconds.")
		log.Println("waiting for other task")
		time.Sleep(10 * time.Second)
	}
}

func (w *Worker) runTask() (task.DockerResult, *task.Task) {
	//responsible for identifying the task’s current state, and then either starting or stopping a task based on the state.

	/* There are two possible scenarios for handling tasks:
	   a) a task is being submitted for the first time, so the Worker will not know about it

	   b) a task is being submitted for the Nth time, where the task submitted represents the desired state to which the current task should transition.

	   The worker will interpret tasks it already has in its Db field as existing tasks, that is tasks that it has already seen at least once. If a task is in the Queue but not the Db, then this is the first time the worker is seeing the task, and we default to starting it.
	*/
	//retrieve task from queue if it exists.
	log.Printf("Running task")
	w.mu.Lock()
	if w.Queue.Len() == 0 {
		w.mu.Unlock()
		return task.DockerResult{}, nil
	}
	t := w.Queue.Dequeue()
	w.mu.Unlock()

	//assert that it's of task.Task type
	taskQueued, ok := t.(*task.Task)
	if !ok {
		log.Printf("Error: Item in queue is not a *task.Task")
		return task.DockerResult{Error: fmt.Errorf("Invalid type in queue")}, nil
	}
	log.Printf("Task %v is in state %v\n", taskQueued.ID, taskQueued.State)

	//validate transition state is correct.
	var result task.DockerResult
	var updatedTask *task.Task

	w.mu.Lock()
	defer w.mu.Unlock()
	switch taskQueued.State {
	case task.Pending:
		taskQueued.State = task.Scheduled
		fallthrough //manually fall through to next case
	case task.Scheduled:
		//if it's scheduled, we want to start the task.
		log.Printf("Starting scheduled task %v\n", taskQueued.ID)
		result, updatedTask = w.StartTask(taskQueued)

		if result.Error != nil {
			log.Printf("Error starting task %v:%v \n", taskQueued.ID, result)
			return result, taskQueued
		}
		log.Printf("Task that was scheduled %v is now in state %v\n", updatedTask.ID, updatedTask.State)
		return result, updatedTask
	case task.Completed:
		//if the task's state from the queue is completed we want to stop the task(and therefore transition it to Completed. )
		result = w.StopTask(taskQueued)
		return result, taskQueued

	default:
		err := fmt.Errorf("unexpected task state: %v", taskQueued.State)
		return task.DockerResult{Error: err}, taskQueued
	}

}

func (w *Worker) StartTask(t *task.Task) (task.DockerResult, *task.Task) {
	//starts a given task.
	log.Printf("StartTask: Beginning for task %v", t.ID)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("StartTask: Recovered from panic in task %v: %v", t.ID, r)
		}
	}()
	//set up necessary fields
	t.StartTime = time.Now().UTC()

	config := t.NewConfig(t)
	d := t.NewDocker(config)
	if d == nil {
		log.Printf("StartTask: Failed to create Docker object for task %v", t.ID)
		t.State = task.Failed
		w.TaskIds[t.ID] = t
		return task.DockerResult{}, nil
	}

	//run the task
	result := d.Run(w.DockerClient)

	if result.Error != nil {
		//log.Printf("Error starting task %v:%v \n", t.ID, result)
		t.State = task.Failed
		w.TaskIds[t.ID] = t
		return result, nil
	}
	log.Printf("Docker container: {%v} started successfully for task %v", t.ContainerID, t.ID)

	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.TaskIds[t.ID] = t


	log.Printf("Started task %v with container ID %v\n", t.ID, t.ContainerID)
	return result, t

}
func (w *Worker) StopTask(t *task.Task) task.DockerResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	log.Printf("Attempting to stop container with ID: %s", t.ContainerID)
	//receives a task and stops it
	config := t.NewConfig(t)
	d := t.NewDocker(config)

	result := d.Stop(w.DockerClient, t.ContainerID)
	if result.Error != nil {
		log.Printf("Error occurred in worker stopping container %v: %v \n", t.ContainerID, result)
	}
	t.EndTime = time.Now().UTC()
	t.State = task.Completed
	w.TaskIds[t.ID] = t
	log.Printf("Stopped and removed container %v for task %v \n", t.ContainerID, result)
	return result
}

// function to get all tasks for this worker
func (w *Worker) GetTasks() []task.Task {
	tasks := make([]task.Task, 0, len(w.TaskIds))
	for _, t := range w.TaskIds {
		tasks = append(tasks, *t)
	}
	return tasks
}

// function to check if task is still running
func (w *Worker) IsTaskRunning(t task.Task, containerID string) (bool, error) {
	if containerID == "" {
		return false, nil
	}
	config := t.NewConfig(&t)
	d := t.NewDocker(config)
	isRunning, err := d.IsRunning(w.DockerClient, containerID)
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
	for {
		// 1. Create a list of tasks to check to avoid holding the lock for long.
		var tasksToInspect []*task.Task
		w.mu.Lock()
		for _, t := range w.TaskIds {
			if t.State == task.Running {
				tasksToInspect = append(tasksToInspect, t)
			}
		}
		w.mu.Unlock()

		log.Printf("Reconciliation loop: worker checking %d running tasks(as in docker container status)", len(tasksToInspect))

		// 2. Now, iterate over the copy, performing slow I/O operations.
		for _, t := range tasksToInspect {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				log.Printf("MonitorTasks: Error inspecting task %s: %v", t.ID, resp.Error)
				// The container might have been removed. Mark as failed.
				w.updateTaskState(t.ID, task.Failed)
				continue
			}

			if resp.Container == nil {
				log.Printf("MonitorTasks: No container info for running task %s", t.ID)
				w.updateTaskState(t.ID, task.Failed)
				continue
			}

			// 3. Check the container's status and update if it has exited.
			if resp.Container.State.Status == "exited" {
				log.Printf("MonitorTasks: Container for task %s has exited.", t.ID)
				w.updateTaskState(t.ID, task.Failed)
			}

			// 4. Update the task's HostPorts with the actual assigned ports.
			w.mu.Lock()
			if taskToUpdate, ok := w.TaskIds[t.ID]; ok {
				taskToUpdate.HostPorts = resp.Container.NetworkSettings.Ports
			}
			w.mu.Unlock()
		}

		// 5. Sleep for a while before the next reconciliation cycle.
		time.Sleep(15 * time.Second)
	}
}
func (w *Worker) updateTaskState(taskID uuid.UUID, newState task.State) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if task, ok := w.TaskIds[taskID]; ok {
		task.State = newState
		task.EndTime = time.Now().UTC()
	}
}

func (w *Worker) InspectTask(t task.Task) task.DockerInspectResponse {
	config := t.NewConfig(&t)
	d := t.NewDocker(config)

	response := d.InspectContainer(w.DockerClient, t.ContainerID)

	return response

}
