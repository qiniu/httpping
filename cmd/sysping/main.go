package main

import (
	"fmt"
	"github.com/longbai/ping/sysping"
)

func main() {
	po, err := sysping.Ping("127.0.0.1", 5, 15, 10)
	fmt.Println(po, err)
}
