package controller

import (
	"elevator/elevio"
	"io"
	"log"
	"os"
)

const hallCallStatePath = ".cabcall_cache"

func restoreRequests(numFloors int) [][3]bool {
	file, err := os.Open(hallCallStatePath)
	if err != nil {
		return make([][3]bool, numFloors)
	}
	defer file.Close()

	content, _ := io.ReadAll(file)

	if len(content) != numFloors {
		log.Printf("Invalid state of `%s`: Not engough floors\n", hallCallStatePath)
		return make([][3]bool, numFloors)
	}

	requests := make([][3]bool, 0, 10)
	for i := range numFloors {
		row := [3]bool{}
		row[elevio.BT_Cab] = content[i] != '0'
		requests = append(requests, row)
	}

	return requests
}

func flushRequests(requests [][3]bool) {
	file, err := os.OpenFile(hallCallStatePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error writing to `%s`\n", hallCallStatePath)
		return
	}
	defer file.Close()

	for _, row := range requests {
		if row[elevio.BT_Cab] {
			file.WriteString("1")
		} else {
			file.WriteString("0")
		}
	}
}
