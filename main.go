package main

import (
	"flag"
	"time"
)

func main() {
	addrPtr := flag.String("addr", "localhost:15657", "Address of elevator hardware")
	flag.Parse()

	elevator := NewElevator(*addrPtr, 9, 1000*time.Millisecond)
	elevator.Run()
}
