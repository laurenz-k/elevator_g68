package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	exercise1()
	// exercise2()
	// exercise3()
	// exercise4()
}

// receive server address via UDP
func exercise1() {
	addr, _ := net.ResolveUDPAddr("udp", ":30000")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, serverAddr, _ := conn.ReadFrom(buf)
		fmt.Printf("Server %s sent: '%s'\n", serverAddr, string(buf[:n]))
	}
}

// send and receive UDP messages
func exercise2() {
	addr, _ := net.ResolveUDPAddr("udp", ":20000")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	receiveMessages := func(conn *net.UDPConn) {
		buffer := make([]byte, 1024)
		for {
			n, remoteAddr, _ := conn.ReadFromUDP(buffer)
			message := string(buffer[:n])
			fmt.Printf("Received %d bytes from %s: %s\n", n, remoteAddr, message)
		}
	}

	sendMessages := func(conn *net.UDPConn) {
		rcvAddr, _ := net.ResolveUDPAddr("udp", "10.100.23.204:20000")
		for {
			_, _ = conn.WriteToUDP([]byte("Hello Word\000"), rcvAddr)
			time.Sleep(2 * time.Second)
		}
	}

	go sendMessages(conn)
	go receiveMessages(conn)

	select {}
}

// connect to tcp echo server
func exercise3() {
	conn, _ := net.Dial("tcp", "10.100.23.204:34933")

	buf := make([]byte, 1024)
	for {
		n, _ := conn.Read(buf)
		fmt.Println(string(buf[:n]))
		conn.Write([]byte("Hello Mr and Mrs Server\000"))

		time.Sleep(2 * time.Second)
	}
}

// tell server to connect
func exercise4() {
	conn, _ := net.Dial("tcp", "10.100.23.204:34933")
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	fmt.Println(string(buf[:n]))

	msg := []byte("Connect to: 10.100.23.28:8069\000")
	conn.Write([]byte(msg))

	go handleConnections()
	select {}
}

func handleConnections() {
	listener, _ := net.Listen("tcp", ":8069")
	defer listener.Close()

	buf := make([]byte, 1024)
	for {
		conn, _ := listener.Accept()
		fmt.Println("Accepted connection")

		n, _ := conn.Read(buf)
		fmt.Println(string(buf[:n]))
	}
}
