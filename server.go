// server.go
package main

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

type client_handle struct {
	//stores informations that allows server to manage clients
	name string
	conn net.Conn
}
type list_message struct {
	a       *client_handle
	proceed chan *list.Element
}

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
	in_cl := make(chan uint64)
	/*synchronizes write_client and read_client*/
	go write_client(c.conn, in_cl) //here we fork to be able to write to client concurrently
	read_client(c.conn, in_cl)     //no need to fork - we will finish when client does not send anything
}

func print_list(l *list.List) {
	for e := l.Front(); nil != e; e = e.Next() {
		fmt.Println(e.Value)
	}
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
	client_list := list.New()
	client_channel := make(chan list_message) //channel that receives updates on client list
	can_kick_ch := make(chan bool)            //signals server we have a sufficiently long list of clients
	go func() {
		/*this anonymous go routine is a client list updater
		whenever it receives a message that list update has to be made, it blocks
		a start of corresponding handle() call until its job is done*/
		i := 0
		for {
			msg := <-client_channel
			element := client_list.PushBack(msg.a)
			msg.proceed <- element
			if i == kick_Client {
				can_kick_ch <- true
			}
			i++
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			select { //if some process has finished we note that
			case <-ticker.C:
				if online < 1 {
					timeout <- true
					break
				}
			case <-q:
				online--
			}
		}
		defer ticker.Stop()
	}()
	for i := 0; i < working_day; i++ { //if there are people to serve OR too few clients have been served
		go func() { //parallel accepts/handles
			conn, err := ln.Accept()
			count++
			if nil != err {
				log.Println(err)
			} else {
				online++                                                    //number of online processes is increased
				proceed := make(chan *list.Element, 1)                      //this line will signal us to proceed once the list of clients is updated
				new_client_handle := client_handle{"None", conn}            //this line is necessary to alter name later in handle() method. somewhat artificial, we don't want to misuse memory so much..
				client_channel <- list_message{&new_client_handle, proceed} //this line signals client list updater to make changes
				element := <-proceed                                        //wait until the necessary list update is done
				handle(&new_client_handle, count, q)
				client_list.Remove(element) //as soon as handle stops we remove client from list
			}
		}()
	}
	<-can_kick_ch //I have many clients, last can be kicked
	fmt.Println("My clients for today were")
	print_list(client_list)
	fmt.Println("-------------------------")
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
	}
	fmt.Println("Now I have only these clients")
	print_list(client_list)
	fmt.Println("-------------------------")
	<-timeout
	quit <- true
}
