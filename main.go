// srvcl_test project main.go
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
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
)

func read_client(c net.Conn, out_chan chan uint64) {
	/* Receives requests from client until client says stop 
	or until number of requests exceeds maximum.
	Processes requests and then send resulting numbers to write_client routine*/
	requests := 0
	buffer := make([]byte, buffer_size)
	var (
		bytes_read int
		err        error
	)
	for bytes_read, err = read_deadline(c, waiting_time, buffer); bytes_read > 0 && requests < max_requests; bytes_read, err = read_deadline(c, waiting_time, buffer) {
		if nil != err {
			log.Println("Unable to read, finalizing")
			break
		}
		requests++
		number, succ := binary.Uvarint(buffer)
		if succ < 1 { //Somehow Uvarint returned not what we wanted to have
			log.Println("Corrupted number in client request.")
			continue
		}
		log.Println("Obtained number", number)
		out_chan <- 2 * number //this should be really (a bit) more complicated
	}
	defer func() {
		close(out_chan) //Now write_client knows it is time to stop too
	}()
}

func write_client(c net.Conn, in_chan chan uint64) {
	/* Sends whatever is ready to the client*/
	var buffer = make([]byte, buffer_size)
	for number := range in_chan {
		succ := binary.PutUvarint(buffer, number)
		if succ < 1 { //Somehow PutUvarint returned something we did not want to have
			log.Println("Corrupted number in server response.")
			buffer = []byte("0")
		}
		log.Println("Sending number", number)
		write_deadline(c, waiting_time, buffer)
	}
}

func read_deadline(c net.Conn, t_dur time.Duration, buffer []byte) (bytes_read int, err error) {
	/*Implements read with specified duration t_dur
	If duration ends, read will fail.*/
	defer func() { //clean up
		c.SetReadDeadline(time.Time{}) //we remove deadline, so that future read calls will work fine
	}()
	err = c.SetReadDeadline(time.Now().Add(t_dur))
	if nil != err {
		log.Println("Failed to set deadline")
		return 0, err
	}
	bytes_read, err = c.Read(buffer)
	if debug > 0 {
		log.Println("(Read) Buffer is", buffer)
	}
	if nil != err {
		log.Println("Failed to read on deadline")
	}
	return bytes_read, err
}

func write_deadline(c net.Conn, t_dur time.Duration, buffer []byte) (bytes_wrote int, err error) {
	/*Implements write with specified duration t_dur*/
	defer func() { //clean up
		c.SetWriteDeadline(time.Time{}) //we remove deadline, so that future write calls will work fine
	}()
	err = c.SetWriteDeadline(time.Now().Add(t_dur))
	if nil != err {
		return 0, err
	}
	if debug > 0 {
		//log.Println("(Written) Buffer is", buffer)
		if 0 == buffer[0] {
			log.Panicln("Zero buffer!")
		}
	}
	bytes_wrote, err = c.Write(buffer)
	if nil != err {
		log.Println("Failed to write in time")
	}
	return bytes_wrote, err
}

func handle(c net.Conn, counter int, q chan bool) {
	/*Reads name of new client and sends current number of clients in return.
	Forks write_client() to process client requests 
	and calls read_client() to obtain requests from client. 
	Concurrency allows client to send multiple requests (total number is limited).*/
	var (
		buffer []byte
		err    error
		name   string
	)
	defer func() { //Finalization
		err = c.Close() //Closes connection as it is handled, no more reading/writing
		if nil != err {
			log.Panicln("Error in closing connection! Reason:", err)
		}
		rec := recover()
		if nil != rec {
			log.Println("Recovered from failure. Client " + strconv.Itoa(counter) + " is abandoned.")
		}
		log.Println("Client", name, "was served.")
		q <- true //Reduces the counter of online clients regardless of function success/failure
	}()
	buffer = make([]byte, buffer_size)
	_, err = read_deadline(c, waiting_time, buffer)
	if nil != err {
		log.Println("Handling error: can't read name!", err)
		name = "Anonymous" + strconv.Itoa(counter)
	} else {
		name = string(buffer)
	}
	//now name of the client is read or assigned to be anonymous in case of unpredicted error
	log.Println("Client", name, "(", counter, ") has requested my help.")
	buffer = make([]byte, buffer_size)
	binary.PutUvarint(buffer, uint64(counter))
	_, err = write_deadline(c, waiting_time, buffer)
	if nil != err {
		log.Println("Handling error: haven't sent information to client", err)
	}
	//now client has received auxiliary information - total number of clients up to date
	in_cl := make(chan uint64)
	/*synchronizes write_client and read_client*/
	go write_client(c, in_cl) //here we fork to be able to write to client concurrently
	read_client(c, in_cl)     //no need to fork - we will finish when client does not send anything
}

func server(quit chan bool) {
	/*Server-side action, listens to clients 
	and initiates handling goroutines upon connection*/
	ln, err := net.Listen(network, port)
	var online, count int = 0, 0
	q := make(chan bool)
	timeout := make(chan bool)
	if nil != err {
		log.Fatal(err)
	}
	defer ln.Close()
	for i := 0; i < working_day; i++ { //if there are people to serve OR too few clients have been served
		ticker := time.NewTicker(5 * time.Second)
		go func() {
			for {
				select { //if some process has finished we note that
				case <-ticker.C:
					if online < 1 {
						timeout <- true
						ticker.Stop()
					}
				case <-q:
					online--
				}
			}
		}()
		go func() { //parallel accepts/handles
			conn, err := ln.Accept()
			count++
			if nil != err {
				log.Println(err)
			} else {
				online++
				handle(conn, count, q)
			}
		}()
	}
	<-timeout
	quit <- true
}

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

func main() {
	quit := make(chan bool)
	go server(quit) //runs server until server says "enough"
	for i := 0; i <= 2*working_day; i++ {
		go client(address, "topo"+strconv.Itoa(i))
	}
	<-quit
	log.Println("My working day is over")
}
