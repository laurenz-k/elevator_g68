package assigner

import (
	"elevator/elevio"
	"net"
	"time"
	"elevator/statesync"
)

const broadcastAddr = "255.255.255.255"
const broadcastPort = "20068"

type Assignment struct {
	ElevatorID int
	Floor      int
	Button     ButtonType
}

/**
 * @brief Calculates the cost of assigning a call to every elevator still alive.
 *
 * @param call The call to be assigned.
 * @param aliveElevators The IDs of the alive elevators.
 * @return The ID of the elevator with the lowest cost.
 */
func Cost(call elevio.ButtonEvent, aliveElevators []int) int {
	lowestcost  := 1000
	lowestcostID := 0
	
	for _, elevatorID := range aliveElevators {
		state := GetState(elevatorID);
		cost := 0
		if state.Floor < call.Floor { //Checks if we are below the floor of the call
			cost += call.Floor - state.Floor //The difference in floors between the elevator and call is added to the cost
			if state.currDirection == elevio.MD_Down { //Checks if we are going in a direction opposite of the call
				cost += 5								
			}
		} 
		else if state.Floor > call.Floor {  //Checks if we are above the floor of the call
			cost += state.Floor - call.Floor
			if state.currDirection == elevio.MD_Up {
				cost += 5
			}
		}
		else { //If we are neither above or below the floor, we are at the floor
			cost = 0 //No cost associated with a call at the same floor
		}
		
		requests := state.getRequests()

		if state.CurrDirection == elevio.MD_Up{ //Checks how many stops we have in the upward direction and associates cost with each stop
			for i := state.Floor; i < len(requests[:][1]) - 1; i++ { //Iterates from floor above you to the top floor 
				if requests[i][0] || requests[i][2]{ //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
					cost += 3
				}
			}
		}

		else if state.CurrDirection == eleevio.MD_Down{ //Checks how many stops we have in the downward direction and associates cost with each stop
			for i := state.Floor - 2; i >= 0; i--{ //Iterates from floor below elevator to the bottom floor 
				if requests[i][1] || requests[i][2]{ //Checks for cab calls or hall calls going upwards at the floor and associates cost with it
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
	if allreadyAssigned(request) {
		return
	}
	//Obtain states of alive elevators, calculate their costs. Lowest cost wins. In a draw, lowest/highest ID wins.
	aliveIDs := GetAliveElevatorIDs()
	//Go through the costs of all elevators in loop with. Lowest wins.

	var winnerElevatorID int = Cost(request, aliveIDs)
	
	addr := broadcastAddr + ":" + broadcastPort
	conn, _ := net.Dial("udp", addr)

	defer conn.Close()

	assignment := Assigment{
		ElevatorID: winnerElevatorID,
		Floor:      request.Floor,
		Button:     request.Button,
	}

	conn.Write(serializeAssignment(assignment))
}

/**
 * @brief Checks if a call is already assigned to an elevator.
 *
 * @param request The call to be checked.
 */
func allreadyAssigned(request elevio.ButtonEvent) bool {
	aliveIDs := GetAliveElevatorIDs()
	for _, elevatorID := range aliveIDs {
		state := GetState(elevatorID)
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
	buf := make([]byte, 128)
	buf = append(buf, byte(assignment.ElevatorID))
	buf = append(buf, byte(assignment.Floor))
	buf = append(buf, byte(int(assignment.Button))) 
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
		ElevatorID: m[0],
		Floor:      m[1],
		Button:     ButtonType(m[3]),
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
		assignment := deserialize(buf[:n])
		if assignment.ElevatorID == thisElevatorID {
			// NOTE laurenzk maybe we could just send ButtonEvent here - then we can handle it same
			// way as regular button press in elevator controller loop
			assignmentChan <- assignment.Button
		}
	}
}


/**
 * @brief Reassigns all tasks assigned to a disconnected elevator to the best available elevator.
 * @param disconnectedID The ID of the disconnected elevator.
 */
func ReassignTasksForDisconnectedElevator(disconnectedID int) {
	// TODO:
	// 1. Retrieve all tasks currently assigned to 'disconnectedID'.
	// 2. Mark them as unassigned or move them into a queue of unassigned tasks.
	// 3. Re-run the assignment logic (cost function) for each of those tasks.
	// 4. Assign them to the best available elevator.
	disconnectedState := GetState(disconnectedID)
	for floor, order := range disconnectedState.GetRequests() { //Goes through each floor, 
		if order[0] {											//looks for hall calls assigned to the dead elevator,
			event := elevio.ButtonEvent{ 						//assigns them to a new one.
				Floor:  floor,
				Button: elevio.ButtonType(BT_HallUp)
			}
			assign(event)
		}
		if order[1] {
			event := elevio.ButtonEvent{
				Floor: floor,
				Button: elevio.Buttontype(BT_HallDown)
			}
			assign(event)
		}
	}
}

// Might not be nessecary - Dont implement yet :)
// If the master is down or busy, or if theres any scenario where we cant immediately assign
// a hall call, we can store it in a FIFO queue. Once the master is ready or a new master
// is elected, we pop from the queue and run the assignment. This way we dont lose any button presses
// (not super important as long as we dont light up the button before losing it!)
//
// Insert a new call into the unassigned queue, to be handled when a master is present.
func AddUnassignedTask(call elevio.ButtonEvent) {
	// TODO:
	// 1. Push the call onto a queue or list of unassigned tasks.
	// 2. Optionally broadcast that a new unassigned task is pending (so the master can pick it up).
}
