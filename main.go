package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time" 
	"github.com/google/uuid"
	"minkube/worker"
	"minkube/task"
	"github.com/golang-collections/collections/queue"
	

)


func main() {
	
    
	host := os.Getenv("MINKUBE_HOST")
	if host == ""{
		host = "localhost"
		log.Fatalf("MINKUBE_HOST is not set")
	}
	portStr := os.Getenv("MINKUBE_PORT")
	if portStr == "" {
		portStr = "8080"
		log.Fatal("MINKUBE_PORT environment variable is not set")
	}else {
		log.Printf("Using port %s", portStr)
	}

	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid MINKUBE_PORT value: %v", err)
	}

	fmt.Printf("Starting Minkube worker on %s:%d\n", host, port)
	w := worker.Worker{
		Queue: *queue.New(),
		TaskIds:    make(map[uuid.UUID]*task.Task),
	}
	api := worker.Api{Address: host, Port: port, Worker: &w}
	go runTasks(&w)
	log.Printf("Starting API server on %s:%d\n", host, port)
	api.Start()
	

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