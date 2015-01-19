package connection

import (
	"encoding/base64"
	"errors"
	"fmt"
	"hash/adler32"
)

const (
	PacketDelimiter = '!'
	CON             = iota
	CONACK
	ACK
	DATA
	PING
	CLOSE
)

type PacketFlags byte

func (f PacketFlags) String() string {
	switch f {
	case CON:
		return "CON"
	case CONACK:
		return "CONACK"
	case ACK:
		return "ACK"
	case DATA:
		return "DATA"
	case PING:
		return "PING"
	default:
		return fmt.Sprintf("%v", byte(f))
	}
}

type Packet struct {
	// Control flags.
	Flags PacketFlags
	// Sequence number.
	Seq byte
	// Optional data.
	Data []byte
}

func Encode(p Packet) string {
	unenclen := 2 + len(p.Data) + 4
	tmpbuff := make([]byte, unenclen, unenclen)
	tmpbuff[0] = byte(p.Flags)
	tmpbuff[1] = p.Seq
	for i := 0; i < len(p.Data); i++ {
		tmpbuff[2+i] = p.Data[i]
	}
	cksumidx := unenclen - 4
	cksum := adler32.Checksum(tmpbuff[0:cksumidx])
	tmpbuff[cksumidx+0] = byte((cksum >> 0) & 0xff)
	tmpbuff[cksumidx+1] = byte((cksum >> 8) & 0xff)
	tmpbuff[cksumidx+2] = byte((cksum >> 16) & 0xff)
	tmpbuff[cksumidx+3] = byte((cksum >> 24) & 0xff)
	return base64.StdEncoding.EncodeToString(tmpbuff) + "!"
}

func Decode(s string) (Packet, error) {
	s = s[0 : len(s)-1]
	rawbytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Packet{}, err
	}
	if len(rawbytes) < 6 {
		return Packet{}, errors.New("Not enough bytes in packet.")
	}
	var wirecksum uint32
	cksumidx := len(rawbytes) - 4
	wirecksum |= uint32(rawbytes[cksumidx+0]) << 0
	wirecksum |= uint32(rawbytes[cksumidx+1]) << 8
	wirecksum |= uint32(rawbytes[cksumidx+2]) << 16
	wirecksum |= uint32(rawbytes[cksumidx+3]) << 24
	return Packet{
		Flags: PacketFlags(rawbytes[0]),
		Seq:   rawbytes[1],
		Data:  rawbytes[2:cksumidx],
	}, nil
}
