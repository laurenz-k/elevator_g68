package assigner

import "elevator/elevio"

func Assign(request elevio.ButtonEvent) {
	// calculates cost
	// broadcast on UDP
}

func ReceiveAssignments(thisElevtorId int) {
	// goroutine to receive assignments
	// if assignment is for this Elevator => send buttonpress controller
}
