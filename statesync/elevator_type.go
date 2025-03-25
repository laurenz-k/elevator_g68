package statesync

import (
	"elevator/elevio"
	"time"
)

// TODO maybe offline makes more sense as module-global variable?
type elevatorState struct {
	id            int
	nonce         int
	currFloor     int
	currDirection elevio.MotorDirection
	request       [][3]bool
	lastSync      time.Time
	offline       bool //changed from online to offline because bool is false by default. will be changed once we figure it out
}

/**
 * @brief Gets the ID of the elevator.
 *
 * @return The ID of the elevator.
 */
func (e *elevatorState) GetID() int {
	return int(e.id)
}

/**
 * @brief Gets the current floor of the elevator.
 *
 * @return The current floor of the elevator.
 */
func (e *elevatorState) GetFloor() int {
	return int(e.currFloor)
}

/**
 * @brief Gets the current direction of the elevator.
 *
 * @return The current direction of the elevator.
 */
func (e *elevatorState) GetDirection() elevio.MotorDirection {
	return e.currDirection
}

/**
 * @brief Gets the requests of the elevator.
 *
 * @return A copy of the requests of the elevator.
 */
func (e *elevatorState) GetRequests() [][3]bool {
	requestsCopy := make([][3]bool, len(e.request))
	for i, requests := range e.request {
		requestsCopy[i][elevio.BT_HallUp] = requests[elevio.BT_HallUp]
		requestsCopy[i][elevio.BT_HallDown] = requests[elevio.BT_HallDown]
		requestsCopy[i][elevio.BT_Cab] = requests[elevio.BT_Cab]
	}
	return requestsCopy
}
