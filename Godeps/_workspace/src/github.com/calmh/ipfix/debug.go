package ipfix

import (
	"log"
	"os"
)

var debug = os.Getenv("IPFIXDEBUG") != ""
var dl = log.New(os.Stderr, "[ipfix] ", log.Lmicroseconds|log.Lmicroseconds)
