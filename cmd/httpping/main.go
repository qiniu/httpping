package main

import (
	"fmt"
	"github.com/longbai/ping/network"
	"net"
	"net/http"
)

type TcpWrapper struct {
	d *net.TCPConn
}

func (t TcpWrapper) Dial(network, addr string) (net.Conn, error) {
	return t.d, nil
}

func main() {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", "www.baidu.com:80")
	fmt.Println(err)

	//DialTCP建立一个TCP连接
	//net参数是"tcp4"、"tcp6"、"tcp"
	//laddr表示本机地址，一般设为nil
	//raddr表示远程地址
	tcpConn, err2 := net.DialTCP("tcp", nil, tcpAddr)
	fmt.Println(err2)
	x := http.Client{Transport: &http.Transport{Dial: TcpWrapper{tcpConn}.Dial}}
	x.Get("www.baidu.com")
	info, err := network.GetsockoptTCPInfo(tcpConn)
	fmt.Printf("%+v %v", info, err)
}
