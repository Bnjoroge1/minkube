package service

type State int

const (

	Ready State  = iota  
	Unhealthy
	Overloaded


)