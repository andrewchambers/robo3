package connection

import (
	"io"
	"sync"
	"testing"
	"fmt"
	"bufio"
	"math/rand"
)


type faultyReader struct {
    rc io.Reader
}

func (fr *faultyReader) Read(b []byte) (int, error) {
	n, err := fr.rc.Read(b)
	for i := 0; i < n; i++ {
		if rand.Uint32() % 100 == 0 {
			b[i] = byte(rand.Int())
		}
	}
	return n, err
}

func TestProto1(t *testing.T) {

	var wg sync.WaitGroup

	wg.Add(2)
    var a,c io.Reader
	a, b := io.Pipe()
	c, d := io.Pipe()
	
	a = &faultyReader{a}
	c = &faultyReader{c}
	
	server := func() {
		l := NewLink(a, d)
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
		l := NewLink(c, b)
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

func TestProto2(t *testing.T) {

	var wg sync.WaitGroup

	wg.Add(2)

	a, b := io.Pipe()
	c, d := io.Pipe()
	
	doWrites := func(rw io.ReadWriter, pong bool) {
	    var err error
	    for i := 0 ; i < 100 ; i++ {
	        if pong {
	            _,err = rw.Write([]byte(fmt.Sprintf("pong%d\n", i)))
	        } else {
	            _,err = rw.Write([]byte(fmt.Sprintf("ping%d\n", i)))
	        }
	        if err != nil {
	            break
	        }
	    }
	}
	
	doReads := func(rw io.ReadWriter, pong bool) {
	    var term string
	    if pong {
	        term = "pong99\n"
	    } else {
	        term = "ping99\n"
	    }
	    rdr := bufio.NewReader(rw)
	    for {
	        s, err := rdr.ReadString('\n')
	        if err != nil {
	            break
	        }
	        //fmt.Println(s)
	        if s == term {
	            break
	        }
	    }
	}
	
	server := func() {
		l := NewLink(a, d)
		conn, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		go doWrites(conn, true)
		doReads(conn, false)
		conn.Close()
		conn.allexited.Wait()
		wg.Done()
	}

	client := func() {
		l := NewLink(c, b)
		conn, err := l.Connect()
		if err != nil {
			t.Fatal(err)
		}
		go doWrites(conn, false)
		doReads(conn, true)
		conn.Close()
		conn.allexited.Wait()
		wg.Done()
	}

	go server()
	go client()
	wg.Wait()
}
