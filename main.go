package main

import (
	"fmt"
	"os"
	"time"

	"minkube/manager"
	"minkube/node"
	"minkube/task"
	"minkube/worker"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/docker/docker/client"
)

func main() {
	t := task.Task {
		ID: uuid.New(),
		Name: "Task1",
		State: task.Pending,
		Image: "Image1",
		Memory: 1024,
		Disk: 1,
	}
	te := task.TaskEvent {
		ID: uuid.New(),
		State: task.Pending,
		Timestamp: time.Now(),
		Task: t,
	}
	//print task and task event
	fmt.Printf("Task: %v \n", t)
	fmt.Printf("Task event: %v \n", te)

	//create workers
	w := worker.Worker {
		Name: "worker1",
		Queue: *queue.New(),
		TaskIds: make(map[uuid.UUID]*task.Task),
	}
	fmt.Println("worker: %v \n", w)
	w.CollectStats()
	w.StartTask(t)
	//w.RunTask()
	w.StopTask(t)

	m := manager.Manager{
		Pending: *queue.New(),
		TaskDb: make(map[uuid.UUID]task.Task),
		EventDb: make(map[string][]task.TaskEvent),
		Workers: []string{w.Name},
		WorkersTaskMap: make(map[string][]uuid.UUID),
		TaskWorkerMap: make(map[uuid.UUID]string),
	 }
	fmt.Printf("manager: %v \n", m)
	m.SelectWorker()
	m.UpdateTasks()
	m.SendWork()

	n := node.Node {
		Name: "Node1",
		Ip: "192.168.1.1",
		Cores: 4,
		Memory: 1024,
		Disk: 25,
		Role: "worker",
	}
	fmt.Printf("node: %v \n", n)

	fmt.Printf("Creating a test container \n")
	dockerTask, createResult := createContainer()
	if createResult.Error != nil {
		fmt.Printf("%v", createResult.Error)
		os.Exit(1)
	}
	fmt.Printf("Created container with ID: %s \n", createResult.ContainerId)
	time.Sleep(time.Second * 5)
	fmt.Printf("Stopping container %s \n", createResult.ContainerId)
	_ = stopContainer(dockerTask)
}

func createContainer() ( *task.Docker, *task.DockerResult) {

		c := task.Config {
			Name: "test-container-{4}",
			Image: "postgres:13",
			Env:  []string{
				"POSTGRES_USER=minkube",
				"POSTGRES_PASSWORD=secret",
			},
		}
		dc, _ := client.NewClientWithOpts(client.FromEnv)
		d := task.Docker {
			Client: dc,
			Config: c,
		}
		result := d.Run()
		if result.Error != nil {
			fmt.Println("%v \n", result.Error)
			return nil, nil
		}
		fmt.Printf("Container %s is running with config %v \n", result.ContainerId)
		return &d, &result
}

func stopContainer(d *task.Docker) *task.DockerResult {
	result := d.Stop(d.ContainerId)
	if result.Error != nil {
		fmt.Printf("%v \n", result.Error)
		return nil
	}
	fmt.Printf("Container %s has been stopped and removed \n", result.ContainerId)
	return &result
}
