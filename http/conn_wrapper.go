package http

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"
)

type TcpWrapper struct {
	ping         func(addr string)
	d            *net.TCPConn
	count        int64
	lastWrite    time.Time
	firstRead    *time.Time
	tlsHandshake time.Duration
	connectStart time.Time
	dnsTime      time.Duration
	tcpHandshake time.Duration
	remoteAddr   *net.TCPAddr
	localAddr    string
	domain       string
	error        string
}

func (t *TcpWrapper) Read(b []byte) (n int, err error) {
	n, err = t.d.Read(b)
	t.count += int64(n)
	if t.firstRead == nil {
		tm := time.Now()
		t.firstRead = &tm
	}
	return
}

func (t *TcpWrapper) Write(b []byte) (n int, err error) {
	n, err = t.d.Write(b)
	t.lastWrite = time.Now()
	return
}

func (t *TcpWrapper) Close() error {
	return t.d.Close()
}

func (t *TcpWrapper) LocalAddr() net.Addr {
	return t.d.LocalAddr()
}

func (t *TcpWrapper) RemoteAddr() net.Addr {
	return t.d.RemoteAddr()
}

func (t *TcpWrapper) SetDeadline(tm time.Time) error {
	return t.d.SetDeadline(tm)
}

func (t *TcpWrapper) SetReadDeadline(tm time.Time) error {
	return t.d.SetReadDeadline(tm)
}

func (t *TcpWrapper) SetWriteDeadline(tm time.Time) error {
	return t.d.SetWriteDeadline(tm)
}

func (t *TcpWrapper) resolve(addrStr string) error {
	dnsStart := time.Now()
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		return err
	}
	t.dnsTime = time.Since(dnsStart)
	t.remoteAddr = addr
	t.domain = strings.Split(addrStr, ":")[0]
	return nil
}

func (t *TcpWrapper) connect() (err error) {
	var localAddr *net.TCPAddr
	if t.localAddr != "" {
		localAddr, err = net.ResolveTCPAddr("tcp", t.localAddr+":0")
		if err != nil {
			return err
		}
	}
	dialer := net.Dialer{
		Timeout:   time.Second,
		LocalAddr: localAddr,
	}
	t.connectStart = time.Now()
	conn, err := dialer.Dial("tcp", t.remoteAddr.String())
	if err != nil {
		return err
	}
	t.tcpHandshake = time.Since(t.connectStart)
	tcpConn, _ := conn.(*net.TCPConn)
	t.d = tcpConn
	return nil
}

func (t *TcpWrapper) Dial(_ context.Context, network, addr string) (conn net.Conn, err error) {
	err = t.resolve(addr)
	if err != nil {
		return nil, err
	}
	if t.d == nil {
		go t.ping(t.remoteAddr.IP.String())
	}
	t.firstRead = nil
	err = t.connect()
	return t, err
}

func (t *TcpWrapper) DialTLS(ctx context.Context, network, addr string) (conn net.Conn, err error) {
	td, err := t.Dial(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	cfg := tls.Config{InsecureSkipVerify: true}
	cl := tls.Client(td, &cfg)
	start := time.Now()
	err = cl.HandshakeContext(ctx)
	if err != nil {
		return nil, err
	}
	t.tlsHandshake = time.Since(start)
	t.firstRead = nil //reset for https
	return cl, nil
}

func (t *TcpWrapper) TTFB() time.Duration {
	return t.firstRead.Sub(t.lastWrite)
}
