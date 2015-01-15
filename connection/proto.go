package connection

import (
	"bufio"
	"io"
	//"net"
	"sync"
	"time"
)

type RoboLink struct {
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

func NewLink(r io.Reader, w io.Writer) *RoboLink {
	l := &RoboLink{
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
		l.packetIn <- p
	}
}

func writePackets(l *RoboLink) {
	for {
		select {
		case p := <-l.packetOut:
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

func (l *RoboLink) Accept() (io.ReadWriter, error) {
	for {
		p := <-l.packetIn
		if p.Flags == CON {
			l.packetOut <- Packet{Flags: ACK}
			p = <-l.packetIn
			if p.Flags == ACK {
				break
			}
		}
	}
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
	return con, nil
}

func sendPings(c *RoboCon) {
	ticker := time.NewTicker(500 * time.Millisecond)
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

func (c *RoboLink) Connect() (io.ReadWriter, error) {
	return nil, nil
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

func (c *RoboCon) Write(p []byte) (int, error) {
	c.writemutex.Lock()
	defer c.writemutex.Unlock()
	sendseqnum := c.seqnum
loop:
	for {
		c.l.packetOut <- Packet{Flags: DATA, Seq: sendseqnum}
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
	return len(p), nil
}
