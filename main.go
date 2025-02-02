package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

const filepath = ".state"

func main() {
	backupPhase()
	primaryPhase()
}

func backupPhase() {
	fmt.Println("---Backup Phase---")
	timeout := 1 * time.Second
	for {
		time.Sleep(timeout)

		fileInfo, err := os.Stat(filepath)
		if errors.Is(err, os.ErrNotExist) || fileInfo.ModTime().Add(timeout).Before(time.Now()) {
			fmt.Println("... timed out")
			return
		}
	}
}

func primaryPhase() {
	fmt.Println("---Primary Phase---")

	fmt.Println("... creating new backup")
	cmd := exec.Command("osascript", "-e", `tell app "Terminal" to do script "go run /Users/laurenz/Documents/1\\ Uni/realtime_programming/elevator_g68/main.go"`)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// recover backup value
	i := uint32(0)
	file, _ := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
	defer file.Close()

	b := make([]byte, 4)
	_, err = file.Read(b)
	if err != nil {
		i = 0
	} else {
		i = binary.LittleEndian.Uint32(b)
	}
	fmt.Printf("... resuming from %d\n", i)

	// now start counting
	bts := make([]byte, 4)
	for {
		i++

		binary.LittleEndian.PutUint32(bts, i)
		file.Seek(0, 0)
		file.Write(bts)

		fmt.Println(i)
		time.Sleep(1 + time.Second)
	}
}
