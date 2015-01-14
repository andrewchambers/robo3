package connection

import (
	"bufio"
	"io"
	//"net"
	"sync"
)

type RoboLink struct {
	r               io.Reader
	w               io.Writer
	controlPacketIn chan Packet
	dataPacketIn    chan Packet
	packetOut       chan Packet
	closeOnce       sync.Once
	// Closed on shutdown, don't send anything to this.
	closed chan struct{}
}

type RoboCon struct {
	l *RoboLink

	state int

	// any data that was acked
	// but did not fit into the read buffer.
	buffermutex sync.Mutex
	buffer      []byte

	seqnummutex sync.Mutex
	seqnum      byte

	eseqnummutex sync.Mutex
	eseqnum      byte
}

func NewLink(r io.Reader, w io.Writer) *RoboLink {
	l := &RoboLink{
		r:               r,
		w:               w,
		controlPacketIn: make(chan Packet),
		dataPacketIn:    make(chan Packet),
		packetOut:       make(chan Packet),
		closed:          make(chan struct{}),
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
		if p.Flags == DATA {
			// XXX this needs to drop packets
			// could be caused by no reader
			l.dataPacketIn <- p
		} else {
			l.controlPacketIn <- p
		}
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
		p := <-l.controlPacketIn
		if p.Flags == CON {
			l.packetOut <- Packet{Flags: ACK}
			p = <-l.controlPacketIn
			if p.Flags == ACK {
				break
			}
		}
	}
	con := &RoboCon{l: l}
	go handleCon(con)
	return con, nil
}

func handleCon(c *RoboCon) {
	p := <-c.l.controlPacketIn
	switch p.Flags {
	case PING:
		c.l.packetOut <- Packet{Flags: PING}
	case ACK:
		if p.Seq == c.eseqnum {
			c.eseqnummutex.Lock()
			c.eseqnum ^= 1
			c.eseqnummutex.Unlock()
		}
	default:
	}

}

func (c *RoboLink) Connect() (io.ReadWriter, error) {
	return nil, nil
}

func (c *RoboCon) Read(p []byte) (n int, err error) {

	for {
		c.buffermutex.Lock()
		defer c.buffermutex.Unlock()

		if len(c.buffer) > 0 {
			n := copy(p, c.buffer)
			if n < len(c.buffer) {
				c.buffer = c.buffer[n:]
			} else {
				c.buffer = nil
			}
			return n, nil
		}

		for {
			p := <-c.l.dataPacketIn
			c.eseqnummutex.Lock()
			eseqnum := c.eseqnum
			c.eseqnummutex.Unlock()
			if p.Seq != eseqnum {
				continue
			}
			c.eseqnummutex.Lock()
			c.eseqnum ^= 1
			c.eseqnummutex.Unlock()

			c.buffer = p.Data
			break
		}
	}
}

func (c *RoboCon) Write(p []byte) (n int, err error) {
	c.seqnummutex.Lock()
	sendseqnum := c.seqnum
	c.seqnummutex.Unlock()
	for {
		c.l.packetOut{Packet{Flags: DATA, Seq: sendseqnum}}
		<- c.ackarrived
		break
		// XXX timeout resend
	}
	return len(p), nil
}
