package controller

import (
	"log"
	"time"

	asg "elevator/assigner"
	"elevator/elevio"
	sts "elevator/statesync"
)

const (
	doorOpenDelay     = 3 * time.Second
	floorPollInterval = 20 * time.Millisecond
)

var _elevatorID int

func StartControlLoop(elevatorID int, driverAddr string, numFloors int) {
	_elevatorID = elevatorID

	buttonEvents := make(chan elevio.ButtonEvent)
	floorEvents := make(chan int)
	obstructionEvents := make(chan bool)
	stopEvents := make(chan bool)
	assignmentEvents := make(chan elevio.ButtonEvent)
	errorEvents := make(chan string)

	asg.Init(elevatorID, assignmentEvents)

	elevator := initializeElevator(elevatorID, driverAddr, numFloors)

	sts.StartStatesync(elevator, buttonEvents, errorEvents)

	// Start polling for events
	go elevio.PollButtons(buttonEvents)
	go elevio.PollFloorSensor(floorEvents)
	go elevio.PollObstructionSwitch(obstructionEvents)
	go elevio.PollStopButton(stopEvents)
	go asg.ReceiveAssignments()
	go elevator.processElevatorErrors(errorEvents)

	// Main event loop
	for {
		select {
		case button := <-buttonEvents:
			elevator.handleButtonPress(button)

		case assignment := <-assignmentEvents:
			elevator.handleAssignment(assignment)

		case floor := <-floorEvents:
			elevator.handleFloorChange(floor, errorEvents)

		case obstruction := <-obstructionEvents:
			elevator.handleDoorObstruction(obstruction, errorEvents)

		case stop := <-stopEvents:
			elevator.handleStopButton(stop)
		}
	}
}

func initializeElevator(id int, driverAddr string, numFloors int) *elevator {
	elevio.Init(driverAddr, numFloors)

	moveToNearestFloor()

	elevator := &elevator{
		id:             id,
		state:          ST_Idle,
		floor:          elevio.GetFloor(),
		direction:      elevio.MD_Stop,
		requests:       restoreRequests(numFloors),
		doorObstructed: false,
	}

	elevator.determineNextDirection(elevio.MD_Stop)
	elevio.SetMotorDirection(elevator.direction)

	updateCabButtonLights(elevator.requests)

	if elevator.requests[elevator.floor][elevio.BT_Cab] {
		elevator.openAndCloseDoor()
	}

	return elevator
}

func moveToNearestFloor() {
	if elevio.GetFloor() == -1 {
		elevio.SetMotorDirection(elevio.MD_Up)
		for elevio.GetFloor() == -1 {
			time.Sleep(floorPollInterval)
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
	}
	elevio.SetFloorIndicator(elevio.GetFloor())
}

func (e *elevator) handleButtonPress(b elevio.ButtonEvent) {
	log.Printf("Pressed button %+v\n", b)

	if b.Button == elevio.BT_Cab {
		e.addRequest(b)
	} else {
		assigneeID := asg.Assign(b)
		if _elevatorID == assigneeID {
			e.addRequest(b)
		}
	}
}

func (e *elevator) handleAssignment(b elevio.ButtonEvent) {
	log.Printf("Received assignment: %+v\n", b)
	e.addRequest(b)
}

func (e *elevator) handleFloorChange(floorNum int, errorChan chan string) {
	log.Printf("floor changed %+v\n", floorNum)

	switch e.state {
	case ST_Moving:
		e.floor = floorNum
		elevio.SetFloorIndicator(floorNum)

		if e.shouldStopOnCurrentFloor() {
			e.openAndCloseDoor()
		}

	case ST_Idle:
		errorChan <- "Unexpected move"
		log.Printf("Stuck in Unexpected move error")
	case ST_DoorOpen:
		errorChan <- "Door open move"
		log.Printf("Stuck in Door open move error")
	}
}

func (e *elevator) handleDoorObstruction(isObstructed bool, errorChan chan string) {
	log.Printf("Door obstruction %+v\n", isObstructed)

	if e.state == ST_DoorOpen {
		e.doorObstructed = isObstructed
		return
	} else {
		errorChan <- "Door obstruction error"
		e.doorObstructed = isObstructed
	}
}

func (e *elevator) handleStopButton(isPressed bool) {
}

func (e *elevator) addRequest(b elevio.ButtonEvent) {
	e.requests[b.Floor][b.Button] = true
	flushRequests(e.requests)

	switch e.state {
	case ST_Idle:
		if e.floor < b.Floor {
			e.state = ST_Moving
			e.direction = elevio.MD_Up
			elevio.SetMotorDirection(e.direction)
		} else if e.floor > b.Floor {
			e.state = ST_Moving
			e.direction = elevio.MD_Down
			elevio.SetMotorDirection(e.direction)
		} else {
			e.openAndCloseDoor()
		}
	case ST_Moving:
		break
	case ST_DoorOpen:
		e.requests[e.floor][elevio.BT_Cab] = false
		e.requests[e.floor][elevio.BT_HallUp] = false
		e.requests[e.floor][elevio.BT_HallDown] = false
	}

	updateCabButtonLights(e.requests)
}

func (e *elevator) openAndCloseDoor() {
	prevDirection := e.direction
	e.state = ST_DoorOpen
	e.direction = elevio.MD_Stop
	elevio.SetMotorDirection(e.direction)

	elevio.SetDoorOpenLamp(true)

	e.clearRequestsOnCurrentFloor(prevDirection)

	updateCabButtonLights(e.requests)
}

func (e *elevator) shouldStopOnCurrentFloor() bool {
	if e.direction == elevio.MD_Up {
		return (e.requests[e.floor][elevio.BT_Cab] ||
			e.requests[e.floor][elevio.BT_HallUp] ||
			!hasRequestAbove(e.floor, e.requests))
	} else if e.direction == elevio.MD_Down {
		return (e.requests[e.floor][elevio.BT_Cab] ||
			e.requests[e.floor][elevio.BT_HallDown] ||
			!hasRequestBelow(e.floor, e.requests))
	}
	return false
}

func hasRequestAbove(currFloor int, requests [][3]bool) bool {
	for f := currFloor + 1; f < len(requests); f++ {
		for btn := 0; btn < len(requests[f]); btn++ {
			if requests[f][btn] {
				return true
			}
		}
	}
	return false
}

func hasRequestBelow(currFloor int, requests [][3]bool) bool {
	for f := 0; f < currFloor; f++ {
		for btn := 0; btn < len(requests[f]); btn++ {
			if requests[f][btn] {
				return true
			}
		}
	}
	return false
}

func (e *elevator) clearRequestsOnCurrentFloor(d elevio.MotorDirection) {
	delay := doorOpenDelay

	// Clear cab requests
	if e.requests[e.floor][elevio.BT_Cab] {
		e.requests[e.floor][elevio.BT_Cab] = false
	}

	// Clear same direction hall calls
	if d == elevio.MD_Up && e.requests[e.floor][elevio.BT_HallUp] {
		e.requests[e.floor][elevio.BT_HallUp] = false
	} else if d == elevio.MD_Down && e.requests[e.floor][elevio.BT_HallDown] {
		e.requests[e.floor][elevio.BT_HallDown] = false
	} else if d == elevio.MD_Stop {
		e.requests[e.floor][elevio.BT_HallUp] = false
		e.requests[e.floor][elevio.BT_HallDown] = false
	}

	flushRequests(e.requests)

	time.AfterFunc(delay, func() {
		e.clearOppositeDirectionRequests(d)
	})
}

func (e *elevator) clearOppositeDirectionRequests(d elevio.MotorDirection) {
	delay := 0 * time.Second
	if d == elevio.MD_Up && !hasRequestAbove(e.floor, e.requests) {
		if e.requests[e.floor][elevio.BT_HallDown] {
			e.requests[e.floor][elevio.BT_HallDown] = false
			delay = doorOpenDelay
		}
	} else if d == elevio.MD_Down && !hasRequestBelow(e.floor, e.requests) {
		if e.requests[e.floor][elevio.BT_HallUp] {
			e.requests[e.floor][elevio.BT_HallUp] = false
			delay = doorOpenDelay
		}
	}
	time.AfterFunc(delay, func() {
		for e.doorObstructed {
			time.Sleep(floorPollInterval)
		}
		elevio.SetDoorOpenLamp(false)

		e.determineNextDirection(d)

		elevio.SetMotorDirection(e.direction)
	})
}

// TODO make a pure function?
func (e *elevator) determineNextDirection(d elevio.MotorDirection) {
	// keeps same direction as long as there's requests in same direction left
	if d == elevio.MD_Up && hasRequestAbove(e.floor, e.requests) {
		e.state = ST_Moving
		e.direction = elevio.MD_Up
	} else if d == elevio.MD_Down && hasRequestBelow(e.floor, e.requests) {
		e.state = ST_Moving
		e.direction = elevio.MD_Down
	} else if hasRequestAbove(e.floor, e.requests) {
		e.state = ST_Moving
		e.direction = elevio.MD_Up
	} else if hasRequestBelow(e.floor, e.requests) {
		e.state = ST_Moving
		e.direction = elevio.MD_Down
	} else {
		e.state = ST_Idle
		e.direction = elevio.MD_Stop
	}
}

func updateCabButtonLights(requests [][3]bool) {
	for f := 0; f < len(requests); f++ {
		elevio.SetButtonLamp(elevio.BT_Cab, f, requests[f][elevio.BT_Cab])
	}

	if len(sts.GetAliveElevatorIDs()) == 1 {
		for f := 0; f < len(requests); f++ {
			elevio.SetButtonLamp(elevio.BT_HallDown, f, requests[f][elevio.BT_HallDown])
			elevio.SetButtonLamp(elevio.BT_HallUp, f, requests[f][elevio.BT_HallUp])
		}
	}
}

func (e *elevator) processElevatorErrors(errorChan chan string) {
	for {
		err := <-errorChan
		switch err {
		case "Unexpected move", "Door open move":
			e.handleUnexpectedMove()

		case "Door obstruction error":
			e.handleDoorObstructionError()

		case "Elevator stuck":
			e.handleElevatorStuck()
		}
	}
}

func (e *elevator) handleUnexpectedMove() {
	sts.TurnOffElevator(e.id)
	if elevio.GetFloor() != -1 {
		e.resetToIdle()
	} else {
		moveToNearestFloor()
		e.openAndCloseDoor()
	}
	sts.TurnOnElevator(e.id)
}

func (e *elevator) handleDoorObstructionError() {
	moveToNearestFloor()
	e.openAndCloseDoor()
}

func (e *elevator) handleElevatorStuck() {
	sts.TurnOffElevator(e.id)
	moveToNearestFloor()
	sts.TurnOnElevator(e.id)
}

func (e *elevator) resetToIdle() {
	e.floor = elevio.GetFloor()
	elevio.SetFloorIndicator(e.floor)
	elevio.SetMotorDirection(elevio.MD_Stop)
	e.state = ST_Idle
}
