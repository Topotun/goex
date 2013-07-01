// cl_list
package main

import (
	"container/list"
	"fmt"
	"log"
	"net"
)

type client_handle struct {
	//stores informations that allows server to manage clients
	name string
	conn net.Conn
}
type list_message struct {
	a       *client_handle
	proceed chan bool
	action  int
}

func print_list(l *list.List) {
	for e := l.Front(); nil != e; e = e.Next() {
		fmt.Println(e.Value)
	}
}

func control_list(client_ch chan list_message, kick_ch chan bool) {
	/*this (go) routine is a client list updater
	whenever it receives a message that list update has to be made, it blocks
	a start of corresponding handle() call until its job is done.
	When time comes, it also deletes client from the list.*/
	i := 0 //artificial counter, related to artificial block
	client_map := make(map[client_handle]*list.Element)
	client_list := list.New()
	for {
		msg := <-client_ch
		switch msg.action {
		case 1: //add
			if nil == msg.a {
				msg.proceed <- true
				log.Println("Null pointer client handle passed, unable to make inclusion in list")
				return
			}
			element := client_list.PushBack(msg.a)
			client_map[*msg.a] = element
			log.Println("Adding client", msg.a.name, "to list, pointer", element)
			/*artificial block for counting clients
			shall be removed ASAP*/
			msg.proceed <- true
			if i == kick_Client {
				kick_ch <- true
			}
			i++
		/*end of artificial block*/
		case 2: //delete
			if nil == msg.a {
				msg.proceed <- true
				log.Println("Null pointer client handle passed, unable to make exclusion from list")
				return
			}
			element := client_map[*msg.a]
			if nil == element {
				log.Println("Client", msg.a.name, "is already gone and cannot be removed")
			} else {
				client_list.Remove(element)
				msg.proceed <- true
			}
		case 3: //print
			fmt.Println("My clients for today were")
			print_list(client_list)
			fmt.Println("-------------------------")
		default:
		}
	}

}
