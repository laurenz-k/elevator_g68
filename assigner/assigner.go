package assigner

import (
	"elevator/elevio"
	"elevator/statesync"
	"log"
	"net"
)

const broadcastAddr = "255.255.255.255"
const broadcastPort = "20068"
const transmissionBatchSize = 10

type Assignment struct {
	ElevatorID int
	Button     elevio.ButtonEvent
}

/**
 * @brief Calculates the cost of assigning a call to every elevator still alive.
 *
 * @param call The call to be assigned.
 * @param aliveElevators The IDs of the alive elevators.
 * @return The ID of the elevator with the lowest cost.
 */
func cost(call elevio.ButtonEvent, aliveElevators []int) int {
	// TODO we should't assign to an elevator that's currently obstructed
	lowestcost := 1000
	lowestcostID := 0

	for _, elevatorID := range aliveElevators {
		state := statesync.GetState(elevatorID)
		cost := 0
		if state.GetFloor() < call.Floor { //Checks if we are below the floor of the call
			cost += call.Floor - state.GetFloor()       //The difference in floors between the elevator and call is added to the cost
			if state.GetDirection() == elevio.MD_Down { //Checks if we are going in a direction opposite of the call
				cost += 5
			}
		} else if state.GetFloor() > call.Floor { //Checks if we are above the floor of the call
			cost += state.GetFloor() - call.Floor
			if state.GetDirection() == elevio.MD_Up {
				cost += 5
			}
		} else { //If we are neither above or below the floor, we are at the floor
			cost = 0 //No cost associated with a call at the same floor
		}

		requests := state.GetRequests()

		if state.GetDirection() == elevio.MD_Up { //Checks how many stops we have in the upward direction and associates cost with each stop
			for i := state.GetFloor(); i < len(requests[:][1])-1; i++ { //Iterates from floor above you to the top floor
				if requests[i][0] || requests[i][2] { //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
					cost += 3
				}
			}
		} else if state.GetDirection() == elevio.MD_Down { //Checks how many stops we have in the downward direction and associates cost with each stop
			for i := state.GetFloor() - 2; i >= 0; i-- { //Iterates from floor below elevator to the bottom floor
				if requests[i][1] || requests[i][2] { //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
					cost += 3
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

/**
 * @brief Asssigns a call to the best suited, alive elevator.
 *
 * @param request The call to be assigned.
 */
func Assign(request elevio.ButtonEvent) {
	//Check if the call is already assigned to an elevator
	if alreadyAssigned(request) {
		log.Printf("Call already assigned")
		return
	}
	//Obtain states of alive elevators, calculate their costs. Lowest cost wins. In a draw, lowest/highest ID wins.
	aliveIDs := statesync.GetAliveElevatorIDs()
	//Go through the costs of all elevators in loop with. Lowest wins.

	winnerElevatorID := cost(request, aliveIDs)
	log.Printf("Assigning call to elevator %d", winnerElevatorID)

	addr := broadcastAddr + ":" + broadcastPort
	conn, err := net.Dial("udp", addr)

	if err != nil {
		log.Printf("Error dialing UDP: %v", err)
		return
	}

	// TODO unpluging ethernet causes segfault => debug tomorrow

	defer conn.Close()

	assignment := Assignment{
		ElevatorID: winnerElevatorID,
		Button:     request,
	}

	// TODO might have to deduplicate by using nonce....
	for range transmissionBatchSize {
		conn.Write(serializeAssignment(assignment))
	}
}

/**
 * @brief Checks if a call is already assigned to an elevator.
 *
 * @param request The call to be checked.
 */
func alreadyAssigned(request elevio.ButtonEvent) bool {
	aliveIDs := statesync.GetAliveElevatorIDs()
	for _, elevatorID := range aliveIDs {
		state := statesync.GetState(elevatorID)
		if state.GetRequests()[request.Floor][int(request.Button)] {
			return true
		}
	}
	return false
}

/**
 * @brief Serializes an assignment into a byte slice.
 *
 * @param assignment The assignment to serialize.
 * @return A byte slice representing the serialized assignment.
 */
func serializeAssignment(assignment Assignment) []byte {
	buf := make([]byte, 0, 128)
	buf = append(buf, uint8(assignment.ElevatorID))
	buf = append(buf, uint8(assignment.Button.Floor))
	buf = append(buf, uint8(assignment.Button.Button))
	return buf
}

/**
 * @brief Deserializes a byte slice into an Assignment.
 *
 * @param m The byte slice containing serialized Assignment data.
 * @return The deserialized Assignment.
 */
func deserializeAssignment(m []byte) Assignment {
	assignment := Assignment{
		ElevatorID: int(m[0]),
		Button: elevio.ButtonEvent{
			Floor:  int(m[1]),
			Button: elevio.ButtonType(int(m[2])),
		},
	}
	return assignment
}

/**
 * @brief Establishes a UDP connection and listens for incoming assignments. When an assignment mathcing the elevator is received, it is deserialized
 * @param assignmentChan The channel to send the received assignment to.
 * @param thisElevatorID The ID of the elevator that should receive the assignment.
 */
func ReceiveAssignments(assignmentChan chan elevio.ButtonEvent, thisElevatorID int) {
	addr, _ := net.ResolveUDPAddr("udp", broadcastAddr+":"+broadcastPort)
	conn, _ := net.ListenUDP("udp", addr)

	defer conn.Close()

	buf := make([]byte, 128)

	for {
		n, _, _ := conn.ReadFromUDP(buf)
		assignment := deserializeAssignment(buf[:n])
		if assignment.ElevatorID == thisElevatorID {
			assignmentChan <- assignment.Button
		}
	}
}
