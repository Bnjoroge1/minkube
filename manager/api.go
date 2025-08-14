package manager

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5"
)


type Api struct {
	Address string
	Port int
	Manager *Manager
	Router *chi.Mux
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	log.Printf("Initializing new router")
	a.Router.Mount("/debug", http.DefaultServeMux)
	a.Router.Route("/tasks", func (r chi.Router)  {
		r.Post("/", a.StartTaskHandler)
		r.Get("/", a.GetTasksHandler)
		r.Route("/{taskID}", func (r chi.Router)  {
			r.Delete("/", a.StopTaskHandler)
		})

	})
	
}

func (a *Api) Start() {
	a.initRouter()
	// Create a server with various timeout settings
	addr := fmt.Sprintf("%s:%d", a.Address, a.Port)
	server := &http.Server{
		Addr: addr,
		// Maximum duration for reading the entire request
		ReadTimeout: 5 * time.Second,
		// Maximum duration for writing the response
		WriteTimeout: 10 * time.Second,
		// Maximum duration for reading the request headers
		ReadHeaderTimeout: 2 * time.Second,
		// Maximum amount of time to wait for the next request when keep-alives are enabled
		IdleTimeout: 120 * time.Second,
		// Handler to use for incoming requests
		Handler: a.Router,
	}
	log.Printf("Starting server on address:port, %s", addr)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}