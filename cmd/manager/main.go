package main

import ( 
	"minkube/internal/manager"
	managerhttp "minkube/internal/manager/http"
	"net/http"
	"strings"
	"time"
	"log"
	"os"
	"os/signal"
	"syscall"
)
	


func main(){
	log.Println("Starting in manager mode...")
	workersStr := os.Getenv("MINKUBE_WORKERS")
	if workersStr == "" {
		log.Fatal("MINKUBE_WORKERS environment variable not set (e.g., 'localhost:9001,localhost:9002')")
	}
	workers := strings.Split(workersStr, ",")
	m := manager.New(workers)

	// The manager needs its own API to accept tasks from users.
	managerApi := managerhttp.Api{
		Address: "localhost",
		Port:    8080, // Manager listens on a different port
		Manager: m,
	}
	go func () {
		log.Println("pprof server:", http.ListenAndServe("localhost:6060", nil))
	}()
	go func (){
		for {go m.HealthCheckWorkers(); time.Sleep(3 *time.Second)}
	}()

	go m.ProcessTasks()
	go m.UpdateTasks()
	go managerApi.Start()


	log.Println("Manager API server started on port 8080")
	// Keep the main function running.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}