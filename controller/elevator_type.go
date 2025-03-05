package controller

import "elevator/elevio"

type stateFSM int

const (
	ST_Idle     stateFSM = 0
	ST_Moving   stateFSM = 1
	ST_DoorOpen stateFSM = 2
)

type elevator struct {
	id             int
	state          stateFSM
	floor          int
	direction      elevio.MotorDirection
	requests       [][3]bool
	doorObstructed bool
}

func (e *elevator) GetID() int {
	return e.id
}

func (e *elevator) GetFloor() int {
	return e.floor
}

func (e *elevator) GetDirection() elevio.MotorDirection {
	return e.direction
}

func (e *elevator) GetRequests() [][2]bool {
	requestsCopy := make([][2]bool, len(e.requests))
	for i, requests := range e.requests {
		requestsCopy[i][elevio.BT_HallUp] = requests[elevio.BT_HallUp]
		requestsCopy[i][elevio.BT_HallDown] = requests[elevio.BT_HallDown]
	}
	return requestsCopy
}
