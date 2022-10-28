package network

import "net"

type TcpWrapper struct {
	d *net.TCPConn
}

func (t TcpWrapper) Dial(network, addr string) (net.Conn, error) {
	return t.d, nil
}

func HttpPing(url string) (*TCPInfo, error) {
	return nil, nil
}
