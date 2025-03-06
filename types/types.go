package types

import "elevator/elevio"

type ElevatorState interface {
	GetID() int
	GetFloor() int
	GetDirection() elevio.MotorDirection
	GetRequests() [][3]bool
}
