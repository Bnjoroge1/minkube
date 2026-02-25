package scheduler

import (
	"minkube/node"
	"minkube/task"

)

type Scheduler interface {
	SelectCandidates(tasks []*task.Task, nodes []*node.Node ) []*node.Node //select nodes to run tasks
	Score() map[*node.Node]float64         //rank the nodes selected.
	Pick() *node.Node                //pick the nodes selected.
}
