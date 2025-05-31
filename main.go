package main

import (
	"fmt"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"log"
	"minkube/task"
	"minkube/worker"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {

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
	api := worker.Api{Address: host, Port: port, Worker: &w}
	log.Printf("API: %v", api.Worker)
	go runTasks(&w)     //run tasks in a goroutine
	go w.MonitorTasks() //monitor tasks in a goroutine
	go w.CollectStats() //collect stats in a goroutine
	log.Printf("Starting API server on %s:%d\n", host, port)
	go api.Start() // Start API server in a goroutine

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
		log.Println("waiting for other task") //written by my girl.
		time.Sleep(10 * time.Second)
	}
}
