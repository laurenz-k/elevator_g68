package main

import "time"

func main() {
	elevator := NewElevator(9, 1000*time.Millisecond)
	elevator.Run()
}
