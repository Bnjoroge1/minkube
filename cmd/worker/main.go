package main

import (
	worker "minkube/internal/worker/service"
	workerhttp "minkube/internal/worker/http"
	workerstats "minkube/internal/worker/stats"
	"log"
	"minkube/internal/task"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)




func main(){
	
	log.Println("Starting in worker mode...")
	host := os.Getenv("MINKUBE_HOST")
	portStr := os.Getenv("MINKUBE_PORT")
	port, _ := strconv.ParseInt(portStr, 10, 64)
	dc, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("Failed to create docker client: %v", err)
	}

	w := worker.Worker{
		Queue:   *queue.New(),
		TaskIds: make(map[uuid.UUID]*task.Task),
		Stats:   &workerstats.Stats{},
		DockerClient: dc,
	}

	workerApi := workerhttp.Api{Address: host, Port: port, Worker: &w}

	go w.RunTasks()
	go w.MonitorTasks()
	go w.CollectStats()
	go workerApi.Start()

	log.Printf("Worker API server started on %s:%d", host, port)
	// Keep the main function running.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}