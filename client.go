// client.go
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
)

func client(addr, name string) {
	/*Starts a client, initiates a connection*/
	conn, err := net.Dial(network, addr+port)
	var succ int
	if err != nil {
		fmt.Println("My name is", name, "I couldn't join the server. I am leaving :(")
		return
	}
	buffer := make([]byte, buffer_size)
	/*First we need to send our name and receive number of clients running*/
	copy(buffer, []byte(name))
	_, err = write_deadline(conn, waiting_time, buffer)
	if nil != err {
		log.Println("Failed to send my name to server")
	}
	_, err = read_deadline(conn, waiting_time, buffer)
	if nil != err {
		log.Println("Failed to receive number of clients")
	} else {
		num_clients, succ := binary.Uvarint(buffer)
		if succ > 0 {
			fmt.Println("My name is", name, num_clients, "clients were served including me")
		}
	}
	/*Now we are sending some numbuffer = make([]byte, buffer_size)ber of requests*/
	for j := 0; j < 2*max_requests; j++ {
		buffer = make([]byte, buffer_size)
		number := uint64(rand.Uint32() % 10000)
		fmt.Println("My name is", name, "I am sending number", number, "on attempt", j)
		binary.PutUvarint(buffer, number)
		_, err = write_deadline(conn, waiting_time, buffer)
		if nil != err {
			log.Println("Failed to write to server")
			continue //we hope to recover in the future
		}
		_, err = read_deadline(conn, waiting_time, buffer)
		if nil != err {
			log.Println("Failed to read from server")
			continue //we hope to recover in the future
		}
		number, succ = binary.Uvarint(buffer)
		if succ < 1 {
			fmt.Println("My name is", name, "I failed to get a sensible answer from server on attempt", j, "!")
		} else {
			fmt.Println("My name is", name, "I have got the number", number, "on attempt", j)
		}
	}
	defer conn.Close()
}