package http

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/qiniu/httpping/network"
)

type TcpWrapper struct {
	ip           string
	verifyHost   bool
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
	rounds       []RoundTime
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
	if t.d != nil {
		return t.d.Close()
	}
	return nil
}

func (t *TcpWrapper) TcpHandshake() time.Duration {
	return t.tcpHandshake
}

func (t *TcpWrapper) TlsHandshake() time.Duration {
	return t.tlsHandshake
}

func (t *TcpWrapper) DnsTime() time.Duration {
	return t.dnsTime
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
	host, port, err := net.SplitHostPort(addrStr)
	if t.d == nil && t.ip != "" {
		if err != nil {
			return err
		}
		addrStr = net.JoinHostPort(t.ip, port)
	}
	dnsStart := time.Now()
	addr, err := net.ResolveTCPAddr("tcp", addrStr)
	if err != nil {
		return err
	}
	t.dnsTime = time.Since(dnsStart)
	t.remoteAddr = addr
	t.domain = host
	return nil
}

const base = 51200

var portNum atomic.Uint64

func newPort() int {
	x := portNum.Add(1) % 12800
	return int(base + x)
}

func (t *TcpWrapper) recordPrev() {
	r := RoundTime{
		Domain:             t.domain,
		Ip:                 t.remoteAddr.IP.String(),
		Port:               t.remoteAddr.Port,
		DnsTimeMs:          uint32(t.dnsTime.Milliseconds()),
		ConnectTimeMs:      uint32(t.tcpHandshake.Milliseconds()),
		TLSHandshakeTimeMs: uint32(t.tlsHandshake.Milliseconds()),
		TtfbMs:             uint32(t.TTFB().Milliseconds()),
		TotalSize:          t.count,
		TotalTimeMs:        time.Now().Sub(t.connectStart).Milliseconds(),
	}
	t.rounds = append(t.rounds, r)
}

func (t *TcpWrapper) connect() (err error) {
	var localAddr *net.TCPAddr
	var randAddr = false
	if t.localAddr != "" {
		if !strings.Contains(t.localAddr, ":") {
			localAddr, err = net.ResolveTCPAddr("tcp", t.localAddr+":0")
			if err != nil {
				return err
			}
		} else {
			localAddr, err = net.ResolveTCPAddr("tcp", t.localAddr)
			if err != nil {
				return err
			}
		}
	} else {
		randAddr = true
	}

dial:
	if randAddr {
		localAddr, err = net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(newPort()))
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
		if randAddr && network.IsEADDRINUSE(err) {
			goto dial
		}
		return err
	}
	t.tcpHandshake = time.Since(t.connectStart)
	tcpConn, _ := conn.(*net.TCPConn)
	t.d = tcpConn
	return nil
}

func (t *TcpWrapper) Dial(_ context.Context, network, addr string) (conn net.Conn, err error) {
	if t.d != nil {
		t.recordPrev()
		_ = t.d.Close()
	}
	err = t.resolve(addr)
	if err != nil {
		return nil, err
	}
	if t.d == nil {
		//go t.ping(t.remoteAddr.IP.String())
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
	cfg := tls.Config{ServerName: strings.Split(addr, ":")[0], InsecureSkipVerify: !t.verifyHost}
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
