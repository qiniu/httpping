package main

import (
	"flag"
	"fmt"

	"github.com/qiniu/httpping/command"
)

func main() {
	host := flag.String("h", "127.0.0.1", "host")
	interval := flag.Int("i", 1, "interval")
	local := flag.String("l", "", "local address for bind ping from")
	flag.Parse()
	po, err := command.Ping(*host, *interval, 15, 2, *local)
	fmt.Println(po, err)
}
