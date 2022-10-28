//go:build amd64 && linux

package main

import (
	"fmt"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "www.baidu.com")
	fmt.Println(err)
	info, err := network.GetsockoptTCPInfo(conn)
	fmt.Println(info, err)
}
