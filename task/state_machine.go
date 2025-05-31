package task

type State int

const (
	Pending   State = iota //this is the initial state, the starting point, for every task
	Scheduled              // a task moves to this state once the manager has scheduled it onto a worker
	Running                //a task moves to this state when a worker successfully starts the task (i.e. starts the container).
	Completed              //a task moves to this state when it completes its work in a normal way (i.e. it does not fail)
	Failed                 //if a task does fail, it moves to this state
)

// list of valid states you can transition to from one.
var stateTransitionMap = map[State][]State{
	Pending:   []State{Scheduled},                  //can only go from pending to scheduled
	Scheduled: []State{Scheduled, Failed, Running}, //can schedule, fail to or successful run a task, or still being scheduled.
	Running:   []State{Running, Completed, Failed}, //task completes or fails or still running.
	Completed: []State{},                           //terminal state
	Failed:    []State{},                           //terminal state.

}

// helper to check if can transition from one state to the other. Literally, just checks if given say a pending state, can i go to this other state?
func containsState(states []State, state State) bool {
	for _, s := range states {
		if s == state {
			return true
		}
	}
	return false
}
