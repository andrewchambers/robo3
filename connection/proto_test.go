package connection

import (
	"io"
	"sync"
	"testing"
)

func TestProto(t *testing.T) {

	var wg sync.WaitGroup

	wg.Add(2)

	a, b := io.Pipe()
	c, d := io.Pipe()
	server := func() {
		l := NewLink(a, d, false)
		conn, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		buff := make([]byte, 12, 12)
		_, err = conn.Read(buff)
		if err != nil {
			t.Fatal(err)
		}
		if string(buff) != "hello there!" {
			t.Fatalf("bad string recieved. <%s>\n", string(buff))
		}
		wg.Done()
	}

	client := func() {
		l := NewLink(c, b, true)
		conn, err := l.Connect()
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Write([]byte("hello there!"))
		if err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}

	go server()
	go client()
	wg.Wait()

}
