//go:build amd64 && linux

package network

type TcpWrapper struct {
	d *net.TCPConn
}

func (t TcpWrapper) Dial(network, addr string) (net.Conn, error) {
	return t.d, nil
}

func HttpPing(url string) (*TCPInfo, error) {
	
}
