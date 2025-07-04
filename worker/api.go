package worker

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
)

type Api struct {
	Address string
	Port    int64
	Worker  *Worker
	Router  *chi.Mux //Mux is basically a multiplexer or request router.
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	log.Printf("initializing router")
	a.Router.Route("/tasks", func(r chi.Router) {
		r.Post("/", a.StartTaskHandler)
		r.Get("/", a.GetTasksHandler)
		r.Route("/{taskID}", func(r chi.Router) {
			r.Delete("/", a.StopTaskHandler) //this makes it really easy to potentially add more verbs to the taskID like PUT, PATCH, etc.
			a.Router.Route("/stats", func(r chi.Router) {
				r.Get("/", a.GetStatsHandler)
			})
		})
	})

}

func (a *Api) Start() {
	a.initRouter()
	http.ListenAndServe(fmt.Sprintf("%s:%d", a.Address, a.Port), a.Router)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", a.Address, a.Port), a.Router)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
