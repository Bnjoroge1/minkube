package task

type State int

const (
	Pending   State = iota //this is the initial state, the starting point, for every task
	Scheduled              // a task moves to this state once the manager has scheduled it onto a worker
	Running                //a task moves to this state when a worker successfully starts the task (i.e. starts the container).
	Completed              //a task moves to this state when it completes its work in a normal way (i.e. it does not fail)
	Failed                 //if a task does fail, it moves to this state
)



