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
	closeOnce sync.Once
	// Closed on shutdown, don't send anything to this.
	closed chan struct{}
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

	closed chan struct{}
}

func NewLink(r io.Reader, w io.Writer, debug bool) *RoboLink {
	l := &RoboLink{
		debug:     debug,
		r:         r,
		w:         w,
		packetIn:  make(chan Packet, 32),
		packetOut: make(chan Packet, 32),
		closed:    make(chan struct{}),
	}
	go readPackets(l)
	go writePackets(l)
	return l
}

func readPackets(l *RoboLink) {
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
		l.packetIn <- p
	}
}

func writePackets(l *RoboLink) {
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
	go handleCon(con)
	go handleTimeouts(con)
	go sendPings(con)
	return con
}

func sendPings(c *RoboCon) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.l.packetOut <- Packet{Flags: PING}
		case <-c.closed:
			return
		}
	}
}

func handleTimeouts(c *RoboCon) {
	// defer c.Close()
	defer close(c.keepalive)
	duration := 5 * time.Second
	timer := time.NewTimer(duration)
	defer timer.Stop() // Might not be needed....
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
	var eseqnum byte
	for {
		p := <-c.l.packetIn
		switch p.Flags {
		case DATA:
			if p.Seq == eseqnum {
				select {
				case c.dataIn <- p.Data:
					eseqnum ^= 1
					c.keepalive <- struct{}{}
				default:
					// drop data packet.
				}
			}
			// Send an pack for the packet, even if its an old one.
			select {
			case c.l.packetOut <- Packet{Flags: ACK, Seq: p.Seq}:
			default:
				// couldn't send ack. oh well.
			}
		case PING:
			c.keepalive <- struct{}{}
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

func (l *RoboLink) Accept() (io.ReadWriter, error) {
	for {
		p := <-l.packetIn
		if p.Flags == CON {
			l.packetOut <- Packet{Flags: CONACK}
			p = <-l.packetIn
			if p.Flags == ACK {
				break
			}
		}
	}
	con := newConnection(l)
	return con, nil
}

func (l *RoboLink) Connect() (io.ReadWriter, error) {
	connected := false
	for i := 0; i < 5; i++ {
		l.packetOut <- Packet{Flags: CON}
		var ack Packet
		select {
		case ack = <-l.packetIn:
		case <-time.After(1 * time.Second):
			continue
		}
		if ack.Flags == CONACK {
			l.packetOut <- Packet{Flags: ACK}
			connected = true
			break
		}
	}
	if !connected {
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
		data = <-c.dataIn
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
		c.l.packetOut <- Packet{Flags: DATA, Seq: sendseqnum, Data: d}
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
