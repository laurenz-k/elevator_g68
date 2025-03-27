package assigner

import (
	"elevator/elevio"
	"elevator/statesync"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

// TODO
// 	clearer separation of concerns
// 	assign can assign directly to self without network
// 	minimal interface

const broadcastAddr = "255.255.255.255"
const broadcastPort = "20068"
const transmissionBatchSize = 10

var _initialized bool
var _elevatorID int
var _nonce int
var _assignmentChan chan elevio.ButtonEvent
var _elevatorNonces map[int]int

func Init(elevatorID int, assignmentChan chan elevio.ButtonEvent) {
	if _initialized {
		fmt.Println("assigner already initialized!")
		return
	}
	_elevatorID = elevatorID
	_nonce = 0
	_assignmentChan = assignmentChan
	_elevatorNonces = make(map[int]int)

	_initialized = true
}

type assignment struct {
	assigneeID int
	assignerID int
	button     elevio.ButtonEvent
	nonce      int
}

// ReceiveAssignments starts listening for assignments for this elevator
// and forwards them to to assignment channel.
func ReceiveAssignments() {
	addr, _ := net.ResolveUDPAddr("udp", broadcastAddr+":"+broadcastPort)
	conn, _ := net.ListenUDP("udp", addr)

	defer conn.Close()

	buf := make([]byte, 128)

	for {
		n, _, _ := conn.ReadFromUDP(buf)
		assignment := deserialize(buf[:n])
		if assignment.assigneeID != _elevatorID {
			continue
		}

		// deduplication
		assignerNonce, exists := _elevatorNonces[assignment.assignerID]
		if !exists || assignerNonce < assignment.nonce {
			_assignmentChan <- assignment.button
			_elevatorNonces[assignment.assignerID] = assignment.nonce
		}
	}
}

// Assign finds the cheapest elevator for handling an `request`. This information gets broadcast.
// Returns the ID of the cheapest elevator.
func Assign(request elevio.ButtonEvent) int {
	assigneeID := cost(request)
	log.Printf("Assigning call to elevator %d", assigneeID)

	addr := broadcastAddr + ":" + broadcastPort
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Printf("UDP error: %v", err)
		return assigneeID
	}
	defer conn.Close()

	assignment := assignment{
		assigneeID: assigneeID,
		assignerID: _elevatorID,
		button:     request,
		nonce:      _nonce,
	}
	_nonce++

	for range transmissionBatchSize {
		conn.Write(serialize(assignment))
	}

	return assigneeID
}

// cost returns ID of best currently availible elevator.
// Returns at least the ID of the initializing elevator.
func cost(call elevio.ButtonEvent) int {
	aliveElevators := statesync.GetAliveElevatorIDs()

	lowestcost := 1000
	lowestcostID := _elevatorID

	for _, elevatorID := range aliveElevators {
		state := statesync.GetState(elevatorID)
		if state == nil {
			continue
		}
		cost := 0
		if state.GetFloor() < call.Floor { //Checks if we are below the floor of the call
			cost += call.Floor - state.GetFloor()       //The difference in floors between the elevator and call is added to the cost
			if state.GetDirection() == elevio.MD_Down { //Checks if we are going in a direction opposite of the call
				cost += 10
			}
		} else if state.GetFloor() > call.Floor { //Checks if we are above the floor of the call
			cost += state.GetFloor() - call.Floor
			if state.GetDirection() == elevio.MD_Up {
				cost += 10
			}
		} else { //If we are neither above or below the floor, we are at the floor
			cost = 0 //No cost associated with a call at the same floor
		}

		requests := state.GetRequests()

		if state.GetDirection() == elevio.MD_Up { //Checks how many stops we have in the upward direction and associates cost with each stop
			for i := state.GetFloor(); i < len(requests[:][1])-1; i++ { //Iterates from floor above you to the top floor
				if requests[i][0] || requests[i][2] { //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
					cost += 5
				}
			}
		} else if state.GetDirection() == elevio.MD_Down { //Checks how many stops we have in the downward direction and associates cost with each stop
			for i := state.GetFloor() - 2; i >= 0; i-- { //Iterates from floor below elevator to the bottom floor
				if requests[i][1] || requests[i][2] { //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
					cost += 5
				}
			}
		}

		if cost < lowestcost {
			lowestcost = cost
			lowestcostID = elevatorID
		}
	}
	return lowestcostID
}

// serializes an assignment into a byte slice.
func serialize(assignment assignment) []byte {
	buf := make([]byte, 0, 128)
	buf = append(buf, uint8(assignment.assigneeID))
	buf = append(buf, uint8(assignment.button.Floor))
	buf = append(buf, uint8(assignment.button.Button))
	buf = append(buf, uint8(assignment.assignerID))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(assignment.nonce))
	return buf
}

// deserializes a byte slice into an Assignment.
func deserialize(m []byte) assignment {
	assignment := assignment{
		assigneeID: int(m[0]),
		button: elevio.ButtonEvent{
			Floor:  int(m[1]),
			Button: elevio.ButtonType(int(m[2])),
		},
		assignerID: int(m[3]),
		nonce:      int(binary.LittleEndian.Uint32(m[4:8])),
	}
	return assignment
}
