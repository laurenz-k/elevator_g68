package main

import (
	"fmt"
	"runtime"
)

var i = 0

const iterationCount = 1000000

func incrementing(c chan int, quit chan int) {
	for range iterationCount {
		c <- 1
	}
	quit <- 1
}

func decrementing(c chan int, quit chan int) {
	for range iterationCount {
		c <- -1
	}
	quit <- 1
}

func main() {
	// sets the max CPU cores used to 2 -> test different values to compare performance
	runtime.GOMAXPROCS(2)

	c := make(chan int)
	quit := make(chan int)

	go incrementing(c, quit)
	go decrementing(c, quit)

	quitCount := 0

loop:
	for {
		select {
		case msg := <-c:
			i += msg
		case <-quit:
			quitCount += 1
			if quitCount >= 2 {
				break loop
			}
		}
	}

	fmt.Println("The magic number is:", i)
}
