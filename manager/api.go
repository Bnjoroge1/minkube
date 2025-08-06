package manager

import (
	"fmt"
	"net/http"
	"log"
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
	
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", a.Address, a.Port), a.Router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}