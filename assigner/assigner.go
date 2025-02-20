package assigner

import (
	"elevator/elevio"
	"net"
	"time"
)

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
	//Check if already assigned
	//Obtain states of alive elevators, calculate their costs. Lowest gets it:
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
