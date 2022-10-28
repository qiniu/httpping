package main

import (
	"flag"
	"fmt"
	"github.com/longbai/ping/sysping"
)

func main() {
	host := flag.String("h", "127.0.0.1", "host")
	interval := flag.Int("i", 1, "interval")
	flag.Parse()
	po, err := sysping.Ping(*host, *interval, 15, 2)
	fmt.Println(po, err)
}
