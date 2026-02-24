package worker

type State int

const (

	Ready State  = iota  
	Unhealthy
	Overloaded


)