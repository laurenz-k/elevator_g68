package assigner

import (
	"elevator/elevio"
	"elevator/statesync"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"reflect"
	"time"
)

const broadcastAddr = "255.255.255.255"
const broadcastPort = "49235"
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
	var conn *net.UDPConn

	for {
		var err error
		addr, _ := net.ResolveUDPAddr("udp", broadcastAddr+":"+broadcastPort)
		conn, err = net.ListenUDP("udp", addr)

		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	defer conn.Close()

	buf := make([]byte, 128)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
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

// Assign finds the cheapest elevator for handling a `request`. This information gets broadcast.
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

// cost returns the ID of the best currently available elevator for the given call.
func cost(call elevio.ButtonEvent) int {
    aliveElevators := statesync.GetAliveElevatorIDs()

    lowestcost := 1000
    lowestcostID := _elevatorID

    for _, elevatorID := range aliveElevators {
        state := statesync.GetState(elevatorID)
        if state == nil || reflect.ValueOf(state).IsNil() {
            continue
        }
        cost := 0
        if state.GetFloor() < call.Floor {
            cost += call.Floor - state.GetFloor() // Add floor difference
            if state.GetDirection() == elevio.MD_Down {
                cost += 10 // Penalty for opposite direction
            }
        } else if state.GetFloor() > call.Floor {
            cost += state.GetFloor() - call.Floor
            if state.GetDirection() == elevio.MD_Up {
                cost += 10 // Penalty for opposite direction
            }
        }

        requests := state.GetRequests()

        if state.GetDirection() == elevio.MD_Up {
            for i := state.GetFloor(); i < len(requests[:][1])-1; i++ {
                if requests[i][0] || requests[i][2] {
                    cost += 5 // Add cost for stops in upward direction
                }
            }
        } else if state.GetDirection() == elevio.MD_Down {
            for i := state.GetFloor() - 2; i >= 0; i-- {
                if requests[i][1] || requests[i][2] {
                    cost += 5 // Add cost for stops in downward direction
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
