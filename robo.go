package main

import (
	"flag"
	"fmt"
	"github.com/andrewchambers/robo/connection"
	"io"
	"net"
)

var port = flag.Int("p", 7080, "port to listen or connect to")
var isServer = flag.Bool("server", false, "Run as robo server")

var a, c io.Reader
var b, d io.Writer

func client() {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		panic(err)
	}
	for {
		tcpconn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		link := connection.NewLink(a, d)
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
	link := connection.NewLink(c, b)
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
		panic("SERVER DONE!")
	}
}

func main() {
	flag.Parse()

	a, b = io.Pipe()
	c, d = io.Pipe()

	go server()
	client()
}
