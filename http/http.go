package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/qiniu/httpping/command"
	"github.com/qiniu/httpping/network"
)

type TcpWrapper struct {
	d            *net.TCPConn
	count        int64
	lastWrite    time.Time
	firstRead    *time.Time
	tlsHandshake time.Duration
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

func (t *TcpWrapper) Dial(_ context.Context, _, _ string) (net.Conn, error) {
	return t, nil
}

func (t *TcpWrapper) DialTLS(ctx context.Context, _, _ string) (net.Conn, error) {
	cfg := tls.Config{InsecureSkipVerify: true}
	cl := tls.Client(t, &cfg)
	start := time.Now()
	err := cl.HandshakeContext(ctx)
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

type HttpInfo struct {
	Code               uint16
	Hops               uint16
	RttMs              uint32
	DnsTimeMs          int64
	ConnectTimeMs      int64
	TLSHandshakeTimeMs int64
	TTFBMs             int64
	ReTransmitPackets  uint32
	Speed              float32 // unit kb/s
	TotalSize          int64
	TotalTimeMs        int64
}

func (h *HttpInfo) String() string {
	t, _ := json.MarshalIndent(h, "", "	")
	return string(t)
}

func readAll(b io.ReadCloser) (err error) {
	d := make([]byte, 512*1024)
	for {
		_, err = b.Read(d)
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func hops(ttl uint) uint16 {
	if ttl <= 64 {
		return uint16(64 - ttl)
	} else if ttl <= 128 {
		return uint16(128 - ttl)
	} else if ttl <= 256 {
		return uint16(256 - ttl)
	} else {
		return uint16(512 - ttl)
	}
}

func copyTcpInfo(h *HttpInfo, t *network.TCPInfo) {
	h.RttMs = t.RttMs
	h.ReTransmitPackets = t.ReTransmitPackets
}

func HttpPingSimple(url string) (*HttpInfo, error) {
	return HttpPingGet(url, true, "")
}

func HttpPingGet(url string, ping bool, srcAddr string) (*HttpInfo, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return HttpPing(req, ping, srcAddr)
}

func HttpPing(req *http.Request, ping bool, srcAddr string) (*HttpInfo, error) {
	pWait := make(chan int, 1)
	var httpInfo HttpInfo
	u := req.URL
	var err error
	if u.Scheme == "" {
		u, err = url.Parse("http://" + u.String())
		if err != nil {
			return nil, err
		}
		req.URL = u
	}

	dnsStart := time.Now()
	addr, err := net.ResolveIPAddr("ip", u.Hostname())
	if err != nil {
		return nil, err
	}
	httpInfo.DnsTimeMs = time.Since(dnsStart).Milliseconds()
	if ping {
		go func() {
			p, err := command.Ping(addr.String(), 1, 5, 1, srcAddr)
			if err == nil && len(p.Replies) != 0 {
				httpInfo.Hops = hops(p.Replies[0].TTL)
			}
			pWait <- 1
		}()
	}

	var localAddr *net.TCPAddr
	if srcAddr != "" {
		localAddr, err = net.ResolveTCPAddr("tcp", srcAddr+":0")
		if err != nil {
			return nil, err
		}
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" || u.Scheme == "" {
			port = "80"
		} else if u.Scheme == "https" {
			port = "443"
		}
	}
	remoteAddr, err := net.ResolveTCPAddr("tcp", addr.String()+":"+port)
	if err != nil {
		return nil, err
	}
	connectStart := time.Now()
	tcpConn, err := net.DialTCP("tcp", localAddr, remoteAddr)
	if err != nil {
		return nil, err
	}
	httpInfo.ConnectTimeMs = time.Since(connectStart).Milliseconds()
	w := TcpWrapper{d: tcpConn}

	client := &http.Client{Transport: &http.Transport{DialContext: w.Dial}}
	if u.Scheme == "https" {
		client = &http.Client{Transport: &http.Transport{DialTLSContext: w.DialTLS}}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	httpInfo.Code = uint16(resp.StatusCode)
	err = readAll(resp.Body)
	if err != nil {
		return nil, err
	}
	endTime := time.Now()
	tcpInfo, err := network.GetSockoptTCPInfo(tcpConn)
	if err != nil {
		return nil, err
	}
	copyTcpInfo(&httpInfo, tcpInfo)
	httpInfo.TotalSize = w.count
	httpInfo.TTFBMs = w.TTFB().Milliseconds()
	httpInfo.TotalTimeMs = endTime.Sub(connectStart).Milliseconds()
	//use last write to calculate download speed to avoid small request that firstRead == endTime
	httpInfo.Speed = float32(float64(w.count) / float64(endTime.Sub(w.lastWrite).Milliseconds()))
	if ping {
		<-pWait
	}
	if u.Scheme == "https" {
		httpInfo.TLSHandshakeTimeMs = w.tlsHandshake.Milliseconds()
	}
	return &httpInfo, nil
}
