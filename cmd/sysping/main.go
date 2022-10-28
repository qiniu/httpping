package main

import (
	"flag"
	"fmt"
)

func main() {
	host := flag.String("h", "127.0.0.1", "host")
	interval := flag.Int("i", 1, "interval")
	flag.Parse()
	po, err := command.Ping(*host, *interval, 15, 2, "")
	fmt.Println(po, err)
}
