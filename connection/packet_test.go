package connection

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestPacket(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create a random packet.
		p1 := Packet{}
		p1.Flags = byte(rand.Int())
		p1.Seq = byte(rand.Int())
		rlen := uint(rand.Int()) % 50
		p1.Data = make([]byte, rlen, rlen)
		for i := 0; i < len(p1.Data); i++ {
			p1.Data[i] = byte(rand.Int())
		}
		s := Encode(p1)
		p2, err := Decode(s)
		if err != nil {
			t.Fatal(err)
		}
		if p1.Flags != p2.Flags {
			t.Fatal("Flags differ.")
		}
		if p1.Seq != p2.Seq {
			t.Fatal("Seqs differ.")
		}
		if bytes.Compare(p1.Data, p2.Data) != 0 {
			t.Fatal("Data differs.")
		}
		s = "foo" + s
		_, err = Decode(s)
		if err == nil {
			t.Fatal("packet should be corrupt.")
		}
	}
}
