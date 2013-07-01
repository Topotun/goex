// srvcl_test project main.go
package main

import (
	"log"
	"strconv"
	"time"
)

const ( //ideally these shall be read as parameters
	buffer_size  = 32 //buffer size
	working_day  = 3  //max # clients
	max_requests = 3  //max # requests per client
	waiting_time = time.Minute
	port         = ":2704"
	network      = "tcp"
	address      = "localhost"
	debug        = 0
	kick_Client  = working_day - 1
)

func main() {
	quit := make(chan bool)
	go server(quit) //runs server until server says "enough"
	for i := 0; i < working_day-1; i++ {
		go client(address, "topo"+strconv.Itoa(i+1), false)
	}
	//go client(address, "topo"+strconv.Itoa(working_day), true)
	<-quit
	log.Println("My working day is over")
}
