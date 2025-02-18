package controller

import "elevator/elevio"

func (e *Elevator) GetID() int {
	return e.id
}

func (e *Elevator) GetFloor() int {
	return e.currentFoor
}

func (e *Elevator) GetDirection() elevio.MotorDirection {
	return e.direction
}

func (e *Elevator) GetRequests() [][2]bool {
	requestsCopy := make([][2]bool, len(e.requests))
	for i, requests := range e.requests {
		requestsCopy[i][elevio.BT_HallUp] = requests[elevio.BT_HallUp]
		requestsCopy[i][elevio.BT_HallDown] = requests[elevio.BT_HallDown]
	}
	return requestsCopy
}
