package main

import (
	"fmt"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"runtime/trace"
	"log"
	"minkube/task"
	"minkube/worker"
	"minkube/manager"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := trace.Start(f); err != nil {
		panic(err)
	}
	defer trace.Stop()

	host := os.Getenv("MINKUBE_HOST")
	if host == "" {
		host = "localhost"
		log.Fatalf("MINKUBE_HOST is not set")
	}
	portStr := os.Getenv("MINKUBE_PORT")
	if portStr == "" {
		portStr = "8080"
		log.Fatal("MINKUBE_PORT environment variable is not set")
	} else {
		log.Printf("Using port %s", portStr)
	}

	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid MINKUBE_PORT value: %v", err)
	}

	fmt.Printf("Starting Minkube worker on %s:%d\n", host, port)
	w := worker.Worker{
		Queue:   *queue.New(),
		TaskIds: make(map[uuid.UUID]*task.Task),
		Stats:   &worker.Stats{},
	}
	workers := []string{fmt.Sprintf("%s:%d", host, port)}
     m := manager.New(workers)


	api := worker.Api{Address: host, Port: port, Worker: &w}
	log.Printf("API: %+v", api.Worker)
	go runTasks(&w)     //run tasks in a goroutine
	go w.MonitorTasks() //monitor tasks in a goroutine
	go w.CollectStats() //collect stats in a goroutine
	log.Printf("Starting API server on %s:%d\n", host, port)
	go api.Start() // Start API server in a goroutine

	for i := 0; i < 3; i++ {
        t := task.Task{
		  ID:uuid.New(),
		  ContainerID: "",
            Name:  fmt.Sprintf("test-container-%d", i),
            State: task.Scheduled,
            Image: "strm/helloworld-http",
        }
        
        m.AddTask(&t)
        m.SendWork()
    }
	// Keep the main function running for linux specifically.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

}

func runTasks(w *worker.Worker) {
	//another goroutine that loops over the queue and runs any existing tasks
	for {
		if w.Queue.Len() != 0 {
			
			result, task := w.RunTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
			if task != nil {
				log.Printf("Task %v is with state %v\n", task.ID, task.State)
			}

		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 10 seconds.")
		log.Println("waiting for other task") 
		time.Sleep(10 * time.Second)
	}
}
