package controller

import (
	"log"
	"time"

	asg "elevator/assigner"
	"elevator/elevio"
	sts "elevator/statesync"
)

func StartControlLoop(id int, driverAddr string, numFloors int) {

	elevator := setup(id, driverAddr, numFloors)

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)
	asg_buttons := make(chan elevio.ButtonEvent)

	go sts.BroadcastState(elevator)
	go sts.ReceiveStates()
	go sts.MonitorFailedSyncs()

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)
	go asg.ReceiveAssignments(asg_buttons, id)

	for {
		select {
		case a := <-drv_buttons:
			elevator.handleButtonPress(a)

		case a := <-asg_buttons:
			elevator.addRequest(a)

		case a := <-drv_floors:
			elevator.handleFloorChange(a)

		case a := <-drv_obstr:
			elevator.handleDoorObstruction(a)

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
		requests:       make([][3]bool, numFloors),
		doorObstructed: false,
	}
	elevator.setCabButtonLights()
	return elevator
}

func (e *elevator) handleButtonPress(b elevio.ButtonEvent) {
	log.Printf("Pressed button %+v\n", b)

	if b.Button == elevio.BT_Cab {
		e.addRequest(b)
	} else {
		asg.Assign(b)
	}
}

func (e *elevator) addRequest(b elevio.ButtonEvent) {
	e.requests[b.Floor][b.Button] = true

	// pause 1 second to ensure 10 heartbeats have been sent before lighting the button
	// TODO issue: causes delay for lighting button => appearance of irresponsiveness
	// TODO solution: blast out state n times => then continue
	// time.Sleep(1 * time.Second)

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

	e.setCabButtonLights()
}

func (e *elevator) handleFloorChange(floorNum int) {
	log.Printf("floor changed %+v\n", floorNum)

	switch e.state {
	case ST_Moving:
		e.floor = floorNum
		elevio.SetFloorIndicator(floorNum)

		if e.stopOnCurrentFloor() {
			e.openAndCloseDoor()
		}

	case ST_Idle:
		panic("Floor changed in state \"ST_Idle\"")

	case ST_DoorOpen:
		panic("Floor changed in state \"ST_DoorOpen\"")
	}
}

func (e *elevator) handleDoorObstruction(isObstructed bool) {
	log.Printf("Door obstruction %+v\n", isObstructed)

	switch e.state {
	case ST_DoorOpen:
		e.doorObstructed = isObstructed

	case ST_Moving:
		panic("Door obstructed in state \"ST_Moving\"")

	case ST_Idle:
		panic("Door obstructed in state \"ST_Idle\"")
	}
}

func (e *elevator) handleStopButton(isPressed bool) {
}

func (e *elevator) openAndCloseDoor() {
	prevDirection := e.direction
	e.state = ST_DoorOpen
	e.direction = elevio.MD_Stop
	elevio.SetMotorDirection(e.direction)

	elevio.SetDoorOpenLamp(true)

	e.clearFloorRequests(prevDirection)

	e.setCabButtonLights()

	time.AfterFunc(1*time.Second, func() {
		for e.doorObstructed {
			time.Sleep(20 * time.Millisecond)
		}
		elevio.SetDoorOpenLamp(false)

		e.setNextDirection(prevDirection)

		elevio.SetMotorDirection(e.direction)
	})
}

func (e *elevator) stopOnCurrentFloor() bool {
	if e.direction == elevio.MD_Up {
		return (e.requests[e.floor][elevio.BT_Cab] ||
			e.requests[e.floor][elevio.BT_HallUp] ||
			!e.hasRequestAbove())
	} else if e.direction == elevio.MD_Down {
		return (e.requests[e.floor][elevio.BT_Cab] ||
			e.requests[e.floor][elevio.BT_HallDown] ||
			!e.hasRequestBelow())
	}
	return false
}

func (e *elevator) hasRequestAbove() bool {
	for f := e.floor + 1; f < len(e.requests); f++ {
		for btn := 0; btn < len(e.requests[f]); btn++ {
			if e.requests[f][btn] {
				return true
			}
		}
	}
	return false
}

func (e *elevator) hasRequestBelow() bool {
	for f := 0; f < e.floor; f++ {
		for btn := 0; btn < len(e.requests[f]); btn++ {
			if e.requests[f][btn] {
				return true
			}
		}
	}
	return false
}

func (e *elevator) clearFloorRequests(d elevio.MotorDirection) {
	// cab requests
	e.requests[e.floor][elevio.BT_Cab] = false

	// same direction calls
	if d == elevio.MD_Up {
		e.requests[e.floor][elevio.BT_HallUp] = false
	} else if d == elevio.MD_Down {
		e.requests[e.floor][elevio.BT_HallDown] = false
	} else if d == elevio.MD_Stop {
		e.requests[e.floor][elevio.BT_HallUp] = false
		e.requests[e.floor][elevio.BT_HallDown] = false
	}

	// opposite direction calls iff there's no more unfilled requests in direction
	if d == elevio.MD_Up && !e.hasRequestAbove() {
		e.requests[e.floor][elevio.BT_HallDown] = false
	} else if d == elevio.MD_Down && !e.hasRequestBelow() {
		e.requests[e.floor][elevio.BT_HallUp] = false
	}
}

func (e *elevator) setNextDirection(d elevio.MotorDirection) {
	// keeps same direction as long as there's requests in same direction left
	if d == elevio.MD_Up && e.hasRequestAbove() {
		e.state = ST_Moving
		e.direction = elevio.MD_Up
	} else if d == elevio.MD_Down && e.hasRequestBelow() {
		e.state = ST_Moving
		e.direction = elevio.MD_Down
	} else if e.hasRequestAbove() {
		e.state = ST_Moving
		e.direction = elevio.MD_Up
	} else if e.hasRequestBelow() {
		e.state = ST_Moving
		e.direction = elevio.MD_Down
	} else {
		e.state = ST_Idle
		e.direction = elevio.MD_Stop
	}
}

func (e *elevator) setCabButtonLights() {
	// cab calls get set here, hall calls get set in statesync
	for f := 0; f < len(e.requests); f++ {
		elevio.SetButtonLamp(elevio.BT_Cab, f, e.requests[f][elevio.BT_Cab])
	}
}
