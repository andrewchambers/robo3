package main

/*
import (
	"flag"
	"github.com/andrewchambers/robo/connection"
	//"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
)

//var lAddress = flag.String("p", "127.0.0.1:8080", "address (to be changed to serial port)")
var isServer = flag.Bool("server", false, "Run as robo server")
var cmd = ""

func server() {

}

func client() int {
		conn, err := net.Dial("tcp", "127.0.0.1:22")
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		config := &ssh.ClientConfig{
			User: "andrew",
			Auth: []ssh.AuthMethod{ssh.Password("1qa2ws3ed")},
		}
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, "foobar", config)
		if err != nil {
			panic(err)
		}
		defer sshConn.Close()
		client := ssh.NewClient(sshConn, chans, reqs)
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			panic(err)
		}
		defer session.Close()
		stderr, err := session.StderrPipe()
		if err != nil {
			panic(err)
		}
		go io.Copy(os.Stderr, stderr)
		stdout, err := session.StdoutPipe()
		if err != nil {
			panic(err)
		}
		go io.Copy(os.Stdout, stdout)
		stdin, err := session.StdinPipe()
		if err != nil {
			panic(err)
		}
		go io.Copy(stdin, os.Stdin)
		err = session.Start(cmd)
		if err != nil {
			panic(err)
		}
		// XXX do we need to wait for the io.copy to finish?
		err = session.Wait()
		if err != nil {
			exiterr := err.(*ssh.ExitError)
			return exiterr.Waitmsg.ExitStatus()
		}
		return 0
}

func main() {
	flag.Parse()
	for _, arg := range flag.Args() {
		cmd += arg + " "
	}
	if cmd == "" {
		os.Exit(1)
	}

	if *isServer {
		server()
	} else {
		os.Exit(client())
	}
}

*/

func main() {

}
