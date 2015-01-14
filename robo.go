package robo

import (
	"flag"
	"github.com/andrewchambers/robo/connection"
)

var lAddress = flag.String("p", 1234, "address (to be changed to serial port)")
var server = flag.Bool("server", false, "Run as robo server")

func main() {
	flag.Parse()
}
