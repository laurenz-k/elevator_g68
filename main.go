package main

import (
	"elevator/controller"
	"flag"
)

func main() {
	idPtr := flag.Int("id", 0, "unique identifier of elevator")
	addrPtr := flag.String("addr", "localhost:15657", "Address of elevator hardware")
	flag.Parse()

	controller.StartControlLoop(*idPtr, *addrPtr, 9)
}
