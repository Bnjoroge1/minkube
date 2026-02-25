package minkube

import "minkube/scheduler"
import "minkube/internal/manager"

func NewManager(s scheduler.Scheduler, workers []string) *manager.Manager {
	return manager.New(s, workers)
}