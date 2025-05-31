package scheduler

type Scheduler interface {
	SelectCandidateNodes() //select nodes to run tasks
	Score()                //rank the nodes selected.
	Pick()                 //pick the nodes selected.
}
