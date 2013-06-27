// ioext.go
package main

import (
	"log"
	"net"
	"time"
)

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
