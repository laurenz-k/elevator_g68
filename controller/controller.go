package controller

import (
	"log"
	"time"

	asg "elevator/assigner"
	"elevator/elevio"
	sts "elevator/statesync"
)

var _elevatorID int

func StartControlLoop(id int, driverAddr string, numFloors int) {
	_elevatorID = id

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)
	asg_buttons := make(chan elevio.ButtonEvent)
	error_chan := make(chan string)

	asg.Init(id, asg_buttons)

	elevator := setup(id, driverAddr, numFloors)

	sts.StartStatesync(elevator, drv_buttons, error_chan)

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go asg.ReceiveAssignments()
	go elevator.handleErrors(error_chan)

	for {
		select {
		case a := <-drv_buttons:
			elevator.handleButtonPress(a)

		case a := <-asg_buttons:
			elevator.handleAssignment(a)

		case a := <-drv_floors:
			elevator.handleFloorChange(a, error_chan)

		case a := <-drv_obstr:
			elevator.handleDoorObstruction(a, error_chan)

		case a := <-drv_stop:
			elevator.handleStopButton(a)
		}
	}
}

func setup(id int, driverAddr string, numFloors int) *elevator {
	elevio.Init(driverAddr, numFloors)

	betweenFloors := elevio.GetFloor() == -1
	if betweenFloors {
		elevio.SetMotorDirection(elevio.MD_Up)
		for elevio.GetFloor() == -1 {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
	}
	elevio.SetFloorIndicator(elevio.GetFloor())

	elevator := &elevator{
		id:             id,
		state:          ST_Idle,
		floor:          elevio.GetFloor(),
		direction:      elevio.MD_Stop,
		requests:       restoreRequests(numFloors),
		doorObstructed: false,
	}

	elevator.setNextDirection(elevio.MD_Stop)
	elevio.SetMotorDirection(elevator.direction)

	setCabButtonLights(elevator.requests)
	return elevator
}

func (e *elevator) handleButtonPress(b elevio.ButtonEvent) {
	log.Printf("Pressed button %+v\n", b)

	if b.Button == elevio.BT_Cab {
		e.addRequest(b)
	}

	assigneeID := asg.Assign(b)
	if _elevatorID == assigneeID {
		e.addRequest(b)
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

		if e.stopOnCurrentFloor() {
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

	switch e.state {
	case ST_DoorOpen:
		e.doorObstructed = isObstructed
	case ST_Moving:
		errorChan <- "Door obstruction moving"
		log.Printf("Stuck in Door obstruction error moving")
		e.doorObstructed = isObstructed
	case ST_Idle:
		errorChan <- "Door obstruction idle"
		log.Printf("Stuck in Door obstruction error idle")
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

	setCabButtonLights(e.requests)
}

func (e *elevator) openAndCloseDoor() {
	prevDirection := e.direction
	e.state = ST_DoorOpen
	e.direction = elevio.MD_Stop
	elevio.SetMotorDirection(e.direction)

	elevio.SetDoorOpenLamp(true)

	e.clearFloorRequests(prevDirection)

	setCabButtonLights(e.requests)
}

func (e *elevator) stopOnCurrentFloor() bool {
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

func (e *elevator) clearFloorRequests(d elevio.MotorDirection) {
	delay := 0 * time.Second
	// cab requests
	if e.requests[e.floor][elevio.BT_Cab] {
		delay = 3 * time.Second
		e.requests[e.floor][elevio.BT_Cab] = false
	}
	flushRequests(e.requests)

	// same direction calls
	if d == elevio.MD_Up {
		if e.requests[e.floor][elevio.BT_HallUp] {
			delay = 3 * time.Second
			e.requests[e.floor][elevio.BT_HallUp] = false
		}
	} else if d == elevio.MD_Down {
		if e.requests[e.floor][elevio.BT_HallDown] {
			delay = 3 * time.Second
			e.requests[e.floor][elevio.BT_HallDown] = false
		}
	} else if d == elevio.MD_Stop {
		e.requests[e.floor][elevio.BT_HallUp] = false
		e.requests[e.floor][elevio.BT_HallDown] = false
		delay = 3 * time.Second
	}
	time.AfterFunc(delay, func() {
		e.clearOtherFloorRequests(d)
	})
}

func (e *elevator) clearOtherFloorRequests(d elevio.MotorDirection) {
	delay := 0 * time.Second
	if d == elevio.MD_Up && !hasRequestAbove(e.floor, e.requests) {
		e.requests[e.floor][elevio.BT_HallDown] = false
		delay = 3 * time.Second
	} else if d == elevio.MD_Down && !hasRequestBelow(e.floor, e.requests) {
		e.requests[e.floor][elevio.BT_HallUp] = false
		delay = 3 * time.Second
	}
	time.AfterFunc(delay, func() {
		for e.doorObstructed {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetDoorOpenLamp(false)

		e.setNextDirection(d)

		elevio.SetMotorDirection(e.direction)
	})
}

// TODO make a pure function?
func (e *elevator) setNextDirection(d elevio.MotorDirection) {
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

func setCabButtonLights(requests [][3]bool) {
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

func (e *elevator) handleErrors(errorChan chan string) {
	myID := e.id
	for {
		err := <-errorChan
		switch err {
		case "Unexpected move", "Door open move":
			sts.TurnOffElevator(myID)
			if elevio.GetFloor() != -1 {
				e.floor = elevio.GetFloor()
				elevio.SetFloorIndicator(e.floor)
				elevio.SetMotorDirection(elevio.MD_Stop)
				e.state = ST_Idle
				sts.TurnOnElevator(myID)
			} else {
				elevio.SetMotorDirection(elevio.MD_Down)
				for elevio.GetFloor() == -1 {
					time.Sleep(20 * time.Millisecond)
				}
				e.openAndCloseDoor()
				sts.TurnOnElevator(myID)
			}
		case "Door obstruction moving":
			sts.TurnOffElevator(myID)
			if elevio.GetFloor() != -1 {
				elevio.SetMotorDirection(elevio.MD_Stop)
			} else {
				elevio.SetMotorDirection(elevio.MD_Down)
				for elevio.GetFloor() == -1 {
					time.Sleep(20 * time.Millisecond)
				}
			}
			e.openAndCloseDoor()
			sts.TurnOnElevator(myID)
		case "Door obstruction idle":
			e.openAndCloseDoor()
		case "Elevator stuck":
			sts.TurnOffElevator(myID)
			for elevio.GetFloor() == -1 {
				time.Sleep(20 * time.Millisecond)
			}
			sts.TurnOnElevator(myID)
		}
	}
}
