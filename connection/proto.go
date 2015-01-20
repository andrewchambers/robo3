package connection

import (
	"bufio"
	"fmt"
	"io"
	//"net"
	"sync"
	"time"
)

type RoboLink struct {
	debug bool

	r         io.Reader
	w         io.Writer
	packetIn  chan Packet
	packetOut chan Packet
	
	// Closed on shutdown, don't send anything to this.
	closeOnce sync.Once
	closed chan struct{}
	
	// RoboCon reads from this chan on creation, and 
	// writes to it after it is cleaned up fully.
	// This means only one connection is allowed at a time.
	isfree chan struct{}
	
}

type RoboCon struct {
	l *RoboLink

	state int

	// any data that read
	// but did not fit into the read buffer.
	readmutex sync.Mutex
	buffer    []byte

	dataIn chan []byte

	seqnummutex sync.Mutex
	seqnum      byte

	ackconfirm  chan bool
	awaitingack chan byte
	keepalive   chan struct{}

	writemutex sync.Mutex
    
    closeOnce sync.Once
	closed chan struct{}
	
	allexited sync.WaitGroup
}

func NewLink(r io.Reader, w io.Writer) *RoboLink {
	l := &RoboLink{
		r:         r,
		w:         w,
		packetIn:  make(chan Packet, 32),
		packetOut: make(chan Packet, 32),
		closed:    make(chan struct{}),
		isfree:     make(chan struct{}, 1),
	}
	l.isfree <- struct{}{}
	go readPackets(l)
	go writePackets(l)
	return l
}

func (l *RoboLink) Close() {
    doClose := func() {
        close(l.closed)
    }
    l.closeOnce.Do(doClose)
}

func (c *RoboCon) Close() {
    doClose := func() {
        close(c.closed)
    }
    c.closeOnce.Do(doClose)
}

func (c *RoboCon) sendPacket(p Packet) error {
    select {
    case c.l.packetOut <- p:
        return nil
    case <- c.closed:
        return fmt.Errorf("connection closed.")
    case <- c.l.closed:
        return fmt.Errorf("link closed.")
    }
}

func (c *RoboCon) readPacket() (Packet, error) {
    select {
    case p := <- c.l.packetIn:
        return p, nil
    case <- c.closed:
        return Packet{},fmt.Errorf("connection closed.")
    case <- c.l.closed:
        return Packet{},fmt.Errorf("link closed.")
    }
}


func readPackets(l *RoboLink) {
    defer l.Close()
	r := bufio.NewReader(l.r)
	for {
		s, err := r.ReadString('!')
		if err != nil {
			return
		}
		p, err := Decode(s)
		if err != nil {
			continue
		}
		if l.debug {
			fmt.Printf("got packet... %v\n", p)
		}
	    select {
		case l.packetIn <- p:
		case <- l.closed:
		    return
		}
	}
}

func writePackets(l *RoboLink) {
	defer l.Close()
	for {
		select {
		case p := <-l.packetOut:
			if l.debug {
				fmt.Printf("sending packet... %v\n", p)
			}
			s := Encode(p)
			_, err := l.w.Write([]byte(s))
			if err != nil {
				return
			}
		case <-l.closed:
			return
		}

	}
}

func newConnection(l *RoboLink) *RoboCon {
	con := &RoboCon{}
	con.l = l
	con.dataIn = make(chan []byte, 32)
	con.closed = make(chan struct{})
	con.keepalive = make(chan struct{})
	con.ackconfirm = make(chan bool)
	con.awaitingack = make(chan byte)
	con.allexited.Add(3)
	go handleCon(con)
	go handleTimeouts(con)
	go sendPings(con)
	go func() {
	    con.allexited.Wait()
	    con.l.isfree <- struct{}{}
	}()
	
	return con
}

func sendPings(c *RoboCon) {
	defer c.allexited.Done()
	defer c.Close()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
	    select {
	        case <- ticker.C:
	        case <- c.closed:
	            return
	    }
		err := c.sendPacket(Packet{Flags: PING})
		if err != nil {
		    return
		}
	}
}

func handleTimeouts(c *RoboCon) {
	duration := 5 * time.Second
	timer := time.NewTimer(duration)
	defer timer.Stop() // Might not be needed....
	defer c.allexited.Done()
	defer c.Close()
	for {
		select {
		case <-c.keepalive:
			timer.Reset(duration)
		case <-timer.C:
			return
		case <-c.closed:
			return
		}
	}
}

func handleCon(c *RoboCon) {
    defer c.allexited.Done()
    defer c.Close()
	var eseqnum byte
	for {
		p,err := c.readPacket()
		if err != nil {
		    return
		}
		
		switch p.Flags {
		case DATA:
			if p.Seq == eseqnum {
				select {
				case c.dataIn <- p.Data:
					eseqnum ^= 1
					c.keepalive <- struct{}{}
			        err = c.sendPacket(Packet{Flags: ACK, Seq: p.Seq})
			        if err != nil {
			            return
			        }
				default:
					// drop data packet.
				}
			}
		case PING:
		    select {
			case c.keepalive <- struct{}{}:
			default:
			    // Nothing...
			}
		case ACK:
			select {
			case wanted := <-c.awaitingack:
				if p.Seq == wanted {
					c.ackconfirm <- true
				} else {
					c.ackconfirm <- false
				}
			default:
				// ignore unsolicited acks.
			}
		default:
		}

	}
}

func (l *RoboLink) Accept() (*RoboCon, error) {
	
	select {
	case <- l.isfree:
	case <- time.After(5 * time.Second):
	    return nil, fmt.Errorf("link busy")
	}
	
	for {
	    var p Packet
	    select {
		case p = <-l.packetIn:
		case <- l.closed:
		    l.isfree <- struct {}{}
		    return nil, fmt.Errorf("link down.")
		}
		if p.Flags == CON {
	        select {
		    case l.packetOut <- Packet{Flags: CONACK}:
		    case <- l.closed:
		        l.isfree <- struct {}{}
		        return nil, fmt.Errorf("link down.")
		    }
	        select {
		    case p = <-l.packetIn:
		    case <- l.closed:
		        l.isfree <- struct {}{}
		        return nil, fmt.Errorf("link down.")
		    }
			if p.Flags == ACK {
				break
			}
		}
	}
	con := newConnection(l)
	return con, nil
}

func (l *RoboLink) Connect() (*RoboCon, error) {
	
	select {
	case <- l.isfree:
	case <-time.After(5 * time.Second):
	    return nil, fmt.Errorf("link busy")
	}
	
	connected := false
	for i := 0; i < 5; i++ {
		l.packetOut <- Packet{Flags: CON}
		var ack Packet
		select {
		case ack = <-l.packetIn:
		case <- l.closed:
		    l.isfree <- struct {}{}
		    return nil, fmt.Errorf("link down.")
		case <-time.After(1 * time.Second):
			continue
		}
		if ack.Flags == CONACK {
	        select {
		    case l.packetOut <- Packet{Flags: ACK}:
		    case <- l.closed:
		        l.isfree <- struct {}{}
		        return nil, fmt.Errorf("link down.")
		    }
			connected = true
			break
		}
	}
	if !connected {
	    l.isfree <- struct {}{}
		return nil, fmt.Errorf("failed to establish connection.")
	}
	ret := newConnection(l)
	return ret, nil
}

func (c *RoboCon) Read(p []byte) (int, error) {
	c.readmutex.Lock()
	defer c.readmutex.Unlock()

	var data []byte

	if len(c.buffer) > 0 {
		data = c.buffer
	} else {
	    select {
		case data = <-c.dataIn:
		case <- c.closed:
		    return 0, fmt.Errorf("connection closed.")
		}
	}
	n := copy(p, data)
	if n < len(data) {
		c.buffer = c.buffer[n:]
	}
	return n, nil
}

func (c *RoboCon) Write(d []byte) (int, error) {
	c.writemutex.Lock()
	defer c.writemutex.Unlock()
	sendseqnum := c.seqnum
loop:
	for {
	    err := c.sendPacket(Packet{Flags: DATA, Seq: sendseqnum, Data: d})
		if err != nil {
		    return 0, err
		} 
		select {
		case c.awaitingack <- sendseqnum:
			waswantedseqnum := <-c.ackconfirm
			if waswantedseqnum {
				break loop
			}
		case <-time.After(100 * time.Millisecond):
			// try and resend.
		}
	}
	// Move onto the next seqnum
	c.seqnum ^= 1
	return len(d), nil
}
