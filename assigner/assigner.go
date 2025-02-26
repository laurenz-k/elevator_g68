package assigner

import (
	"elevator/elevio"
	"net"
	"time"
)


// NOTE THAT THIS DOES NOT ALL HAVE TO BE DONE HERE, IF DONE AT ALL. THIS IS JUST A SUGGESTION FOR HOW TO IMPLEMENT IT.

// Assignment should only be done by the master - all non-master elevators should listen for assignments
// and should tell the master about new hall calls, instead of evaluating them themselves
// TODO: Add a function to notify the master about new hall calls, and make the master be aware of this
// TODO: Add a voting system for master election (RAFT)
// If an elevator gets disconnected from the system, its tasks will be reassigned. It should however still try to complete its tasks?
// If an elevator is disconnected from the system, it should still take tasks and assign them to itself, maybe? otherwise it can just sit idle.

// Potentially, add a list of unassigned tasks, where we can keep tasks that have not yet been assigned but where the button has been pressed. 
// If an elevator goes idle, we can just put all its orders in here. It also allows us to use unsigned integers for our table, which is nice.
// Table: 0 = no order, All numbers > 0 = elevator ID, Unnassigned orders are stored in a FIFO queue or something.

// Let me know if im missing something, i will add it ASAP :) - Hlynur


var assignmentChan chan Assignment
		
const broadcastAddr = "255.255.255.255"
const broadcastPort = "20068"

type Assignment struct {
	ElevatorID int
	Floor int
	Button ButtonType
}

//Calculates the cost of each alive elevator and returns the ID of the lowest cost
//Draws -> lowest/highest ID wins perhaps?
func cost(a Assigment) int {
	aliveIDs := GetAliveElevatorIDs()
	return pass
}

func Assign(request elevio.ButtonEvent) {
	// TODO, Jakob, make cost function
	//Check if already assigned to an alive elevator - if so, do nothing
	//Obtain states of alive elevators, calculate their costs. Lowest cost wins. In a draw, lowest/highest ID wins.
	aliveIDs := GetAliveElevatorIDs()
	//Go through the costs of all elevators in loop with. Lowest wins.

	var winnerElevatorId int 

	addr := broadcastAddr + ":" + broadcastPort
	conn, _ := net.Dial("udp", addr)
	
	defer conn.Close()
	//

	asssignment := Assigment{
		ElevatorID: winnerElevatorId,
		Floor: request.Floor,
		Button: request.Button,
	}

	conn.Write(serializeAssignment(assignment))
}

func serializeAssignment(assignment Assignment) []byte {
	buf := make([]byte, 128)
	buf = append(buf, byte(assignment.ElevatorId))
	buf = append(buf, byte(assignment.Floor))
	buf = append(buf, byte(assignment.Button))
	return buf
}

func deserializeAssignment(m byte[]) Assignment {
	assignment := Assignment{
		ElevatorID: m[0],
		Floor: m[1],
		Button: m[3],
	}
	return assignment
}

func ReceiveAssignments(thisElevtorId int) {
	addr, _ := net.ResolveUDPAddr("udp", broadcastAddr + ":" + broadcastPort)
	conn, _ := net.ListenUDP("udp", addr)
	
	defer conn.Close()
	
	buf := make([]byte, 128)
	
	for {
		n, _, _ := conn.ReadFromUDP(buf)
		assignment := deserialize(buf[:n])
		if assignment.ElevatorId == thisElevtorId {
			assignmentChan <- assignment
		}
	}
	
}
