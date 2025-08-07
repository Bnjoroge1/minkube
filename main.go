package main

import (
	"log"
	"minkube/manager"
	"minkube/task"
	"minkube/worker"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	  _ "net/http/pprof"

	"runtime/trace"

	"github.com/docker/docker/client"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
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

	role := os.Getenv("MINKUBE_ROLE")
	if role == "" {
		log.Fatal("MINKUBE_ROLE environment variable is not set (e.g., 'manager' or 'worker')")
	}

	switch role {
	case "manager":
		runManager()
	case "worker":
		runWorker()
	default:
		log.Fatalf("Unknown role: %s", role)
	}
}

func runManager() {
	log.Println("Starting in manager mode...")
	workersStr := os.Getenv("MINKUBE_WORKERS")
	if workersStr == "" {
		log.Fatal("MINKUBE_WORKERS environment variable not set (e.g., 'localhost:9001,localhost:9002')")
	}
	workers := strings.Split(workersStr, ",")
	m := manager.New(workers)

	// The manager needs its own API to accept tasks from users.
	managerApi := manager.Api{
		Address: "localhost",
		Port:    8080, // Manager listens on a different port
		Manager: m,
	}

	go m.ProcessTasks()
	go m.UpdateTasks()
	go managerApi.Start()

	log.Println("Manager API server started on port 8080")
	// Keep the main function running.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

func runWorker() {
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
		Stats:   &worker.Stats{},
		DockerClient: dc,
	}

	workerApi := worker.Api{Address: host, Port: port, Worker: &w}

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


