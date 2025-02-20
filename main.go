package main

import (
	"elevator/controller"
	"flag"
	"time"
)

func main() {
	addrPtr := flag.String("addr", "localhost:15657", "Address of elevator hardware")
	flag.Parse()

	elevator := controller.NewElevator(*addrPtr, 9, 1000*time.Millisecond)
	elevator.Run()
}
