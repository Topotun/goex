// server.go
package main

import (
	"encoding/binary"
	"log"
	"net"
	"strconv"
	"time"
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

func write_client(c net.Conn, in_chan chan uint64, can_finish chan bool) {
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
	can_finish <- true
}

func handle(c *client_handle, counter int, q chan bool) {
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
		err = c.conn.Close() //Closes connection as it is handled, no more reading/writing
		if nil != err {
			log.Println("Error in closing connection! Reason:", err)
		}
		rec := recover()
		if nil != rec {
			log.Println("Recovered from failure. Client " + strconv.Itoa(counter) + " is abandoned.")
		}
		log.Println("Client", name, "was served.")
		q <- true //Reduces the counter of online clients regardless of function success/failure
	}()
	buffer = make([]byte, buffer_size)
	_, err = read_deadline(c.conn, waiting_time, buffer)
	if nil != err {
		log.Println("Handling error: can't read name!", err)
		name = "Anonymous" + strconv.Itoa(counter)
	} else {
		name = string(buffer)
	}
	//now name of the client is read or assigned to be anonymous in case of unpredicted error
	log.Println("Client", name, "(", counter, ") has requested my help.")
	c.name = name //it's ok with assignment, name is never used afterwards
	buffer = make([]byte, buffer_size)
	binary.PutUvarint(buffer, uint64(counter))
	_, err = write_deadline(c.conn, waiting_time, buffer)
	if nil != err {
		log.Println("Handling error: haven't sent information to client", err)
	}
	//now client has received auxiliary information - total number of clients up to date
	in_cl := make(chan uint64, max_requests)
	/*synchronizes write_client and read_client*/

	can_finalize := make(chan bool, 1)
	go write_client(c.conn, in_cl, can_finalize) //here we fork to be able to write to client concurrently
	read_client(c.conn, in_cl)                   //no need to fork - we will finish when client does not send anything
	<-can_finalize
}

func server(quit chan bool) {
	/*Server-side action, listens to clients 
	and initiates handling goroutines upon connection*/
	ln, err := net.Listen(network, port)
	var online, count int = 0, 0
	onl_block_ch, count_block_ch := make(chan bool, 1), make(chan bool, 1)
	onl_block_ch <- true   //opens access to online counter
	count_block_ch <- true //opens access to count counter

	q := make(chan bool, 1)

	timeout := make(chan bool, 1)
	if nil != err {
		log.Fatal(err)
	}
	defer ln.Close()

	client_channel := make(chan list_message) //channel that receives updates on client list
	can_kick_ch := make(chan bool)            //signals server we have a sufficiently long list of clients

	go control_list(client_channel, can_kick_ch)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select { //if some process has finished we note that
			case <-ticker.C:
				if online < 1 {
					timeout <- true
					log.Println("Timeout is reset")
					break
				}
			case <-q:
				dec_counter(online, onl_block_ch)
				log.Println("Decreased online to", online)
			}
		}
		defer ticker.Stop()
	}()
	var last_client *client_handle
	for i := 0; i < working_day; i++ { //if there are people to serve OR too few clients have been served
		go func() { //parallel accepts/handles
			conn, err := ln.Accept()
			inc_counter(count, count_block_ch)
			if nil != err {
				log.Println(err)
			} else {
				inc_counter(online, onl_block_ch)                              //number of online processes is increased
				proceed := make(chan bool)                                     //this line will signal us to proceed once the list of clients is updated
				new_client_handle := client_handle{"None", conn}               //this line is necessary to alter name later in handle() method. somewhat artificial, we don't want to misuse memory so much..
				client_channel <- list_message{&new_client_handle, proceed, 1} //this line signals client list updater to make changes
				<-proceed                                                      //wait until the necessary list update is done
				/*artificial block aimed on kicking the last client*/
				if count == kick_Client+1 {
					last_client = &new_client_handle
					log.Println("Marking client", last_client)
				} /*end of artificial block*/
				handle(&new_client_handle, count, q)                           //handle the connection
				client_channel <- list_message{&new_client_handle, proceed, 2} //as soon as handle stops we remove client from list
				<-proceed
			}
		}()
	}
	//<-can_kick_ch //I have many clients, last can be kicked
	//client_channel <- list_message{nil, nil, 3}
	/*proceed := make(chan bool)
	for nil == last_client {
		time.Sleep(time.Second)
	}
	client_channel <- list_message{last_client, proceed, 2}
	<-proceed*/
	/*
		if kick_Client > 0 {
			i := 0
			e := client_list.Front()
			for ; i < kick_Client; e = e.Next() { //removes kick_Client# client from the list and terminates its connection
				if nil == e {
					break
				}
				if i == kick_Client {
					client_list.Remove(e)
					break
				}
				i++
			}
			if nil != e { //if we found the client handle
				a_handle, ok := e.Value.(*client_handle)
				if ok {
					log.Println("I had enough of client", a_handle.name)
					a_handle.conn.Close()
				} else {
					log.Println("Error: invalid client handle in the list, entry", i)
				}
			}
		}*/
	//client_channel <- list_message{nil, nil, 3}
	<-timeout
	quit <- true
}
