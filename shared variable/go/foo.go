package main

import (
	"fmt"
	"runtime"
	"time"
)

var i = 0

const iterationCount = 1000000

func incrementing() {
	for range iterationCount {
		i++
	}
}

func decrementing() {
	for range iterationCount - 1 {
		i--
	}
}

func main() {
	// sets the max CPU cores used to 2 -> test different values to compare performance
	runtime.GOMAXPROCS(2)

	go incrementing()
	go decrementing()

	time.Sleep(500 * time.Millisecond)
	fmt.Println("The magic number is:", i)
}
