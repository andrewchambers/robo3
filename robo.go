package main

import (
	"flag"
	"fmt"
	"github.com/andrewchambers/robo/connection"
	"io"
	"net"
	"os"
)

var port = flag.Int("p", 7080, "Port to listen on or connect to.")
var isServer = flag.Bool("server", false, "Run as robo server.")

func client() {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		panic(err)
	}
	serport, err := os.OpenFile("/dev/ttyUSB0", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	for {
		tcpconn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		defer serport.Close()
		link := connection.NewLink(serport, serport)
		conn, err := link.Connect()
		if err != nil {
			panic(err)
		}
		done := make(chan struct{})
		go func() {
			io.Copy(tcpconn, conn)
			done <- struct{}{}
			tcpconn.Close()
			conn.Close()
		}()
		go func() {
			io.Copy(conn, tcpconn)
			done <- struct{}{}
			tcpconn.Close()
			conn.Close()
		}()
		<-done
		<-done
		panic("CLIENT DONE!")
	}
}

func server() {
	serport, err := os.OpenFile("/dev/ttyUSB1", os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	link := connection.NewLink(serport, serport)
	for {
		conn, err := link.Accept()
		if err != nil {
			panic(err)
		}
		tcpconn, err := net.Dial("tcp", "127.0.0.1:22")
		if err != nil {
			panic(err)
		}
		done := make(chan struct{})
		go func() {
			_, err := io.Copy(tcpconn, conn)
			panic(err)
			done <- struct{}{}
			tcpconn.Close()
			conn.Close()
		}()
		go func() {
			_, err := io.Copy(conn, tcpconn)
			panic(err)
			done <- struct{}{}
			tcpconn.Close()
			conn.Close()
		}()
		<-done
		<-done
		panic("SERVER DONE!")
	}
}

func main() {
	flag.Parse()
	if *isServer {
		server()
	} else {
		client()
	}
}
