package roundrobin

import (
	"minkube/node"
	"minkube/task"
)

type RoundRobin struct {}

func (r *RoundRobin) SelectCandidates(tasks []*task.Task, nodes []*node.Node) []*node.Node { return []*node.Node{} }
func (r *RoundRobin) Score() map[*node.Node]float64  { return map[*node.Node]float64{}  }
func (r *RoundRobin) Pick() *node.Node               {  return nil}