package http

import (
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"net/http"
	"net/url"
	"time"
	"unsafe"

	"github.com/qiniu/httpping/command"
	"github.com/qiniu/httpping/network"
)

type Pinger struct {
	Req           *http.Request
	SysPing       bool
	SrcAddr       string
	ServerSupport bool
	BodyHasher    hash.Hash
	Redirect      bool
	Timeout       time.Duration
}

type Info struct {
	Server             network.TCPInfo
	Client             network.TCPInfo
	Domain             string
	Ip                 string
	Port               int
	Code               int
	Hops               uint32
	DnsTimeMs          uint32
	ConnectTimeMs      uint32
	TLSHandshakeTimeMs uint32
	TtfbMs             uint32
	ReTransmitPackets  uint32
	Speed              float32 // unit kb/s
	TotalSize          int64
	TotalTimeMs        int64
	Error              string
	PingError          string
	Hash               string
	Loss               float32
}

func (h *Info) String() string {
	t, _ := json.MarshalIndent(h, "", "	")
	return string(t)
}

func minInt(x, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}

func readN(b io.ReadCloser, toRead int, hasher hash.Hash) (err error) {
	d := make([]byte, 64*1024)
	var n int
	for {
		need := minInt(len(d), toRead)
		n, err = b.Read(d[:need])
		if hasher != nil && n > 0 {
			hasher.Write(d[:n])
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}

		toRead -= n
		if toRead <= 0 {
			return
		}
	}
	return
}

const (
	infoSize = int(unsafe.Sizeof(network.TCPInfo{}))
)

func dealWithServerTcpInfo(b io.ReadCloser, contentLength int64, tcpInfo *network.TCPInfo) (err error) {
	err = readN(b, int(contentLength)-infoSize, nil)
	if err != nil {
		return
	}
	d := (*[infoSize]byte)(unsafe.Pointer(tcpInfo))[:]
	_, err = io.ReadFull(b, d)
	return
}

func readAll(b io.ReadCloser, hasher hash.Hash) (err error) {
	d := make([]byte, 64*1024)
	var n int
	for {
		n, err = b.Read(d)
		if hasher != nil && n > 0 {
			hasher.Write(d[:n])
		}
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func hops(ttl uint) uint32 {
	if ttl <= 64 {
		return uint32(64 - ttl)
	} else if ttl <= 128 {
		return uint32(128 - ttl)
	} else if ttl <= 256 {
		return uint32(256 - ttl)
	} else {
		return uint32(512 - ttl)
	}
}

func PingSimple(url string) (*Info, error) {
	return PingGet(url, true, "")
}

func PingGet(url string, ping bool, srcAddr string) (*Info, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return Ping(req, ping, srcAddr)
}

func sysPing(httpInfo *Info, addr, srcAddr string, wait chan<- int) {
	p, err := command.Ping(addr, 1, 5, 1, srcAddr)
	if err == nil {
		if len(p.Replies) != 0 {
			httpInfo.Hops = hops(p.Replies[0].TTL)
		} else {
			httpInfo.PingError = "ping wait more than 5s"
		}
	} else {
		httpInfo.PingError = err.Error()
	}
	wait <- 1
}

func (p *Pinger) Ping() (*Info, error) {
	pWait := make(chan int, 1)
	var httpInfo Info
	u := p.Req.URL
	var err error
	if u.Scheme == "" {
		u, err = url.Parse("http://" + u.String())
		if err != nil {
			return nil, err
		}
		p.Req.URL = u
	}

	w := &TcpWrapper{}

	if p.SysPing {
		w.ping = func(addr string) {
			sysPing(&httpInfo, addr, p.SrcAddr, pWait)
		}
	}

	err = p.do(&httpInfo, w)
	if err != nil {
		return &httpInfo, nil
	}

	endTime := time.Now()
	httpInfo.TotalSize = w.count
	httpInfo.TotalTimeMs = endTime.Sub(w.connectStart).Milliseconds()
	//use last write to calculate download speed to avoid small request that firstRead == endTime
	t := endTime.Sub(w.lastWrite).Milliseconds() - int64(httpInfo.Client.RttMs)
	if t <= 0 {
		t = 1
	}
	httpInfo.Speed = float32(float64(w.count) / float64(t))
	if p.SysPing {
		<-pWait
	}
	if p.BodyHasher != nil {
		httpInfo.Hash = hex.EncodeToString(p.BodyHasher.Sum(nil))
	}

	return &httpInfo, nil
}

func (p *Pinger) do(httpInfo *Info, w *TcpWrapper) error {
	client := &http.Client{
		Transport: &http.Transport{DialContext: w.Dial, DialTLSContext: w.DialTLS},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if p.Redirect {
				return nil
			}
			return http.ErrUseLastResponse
		}, Timeout: p.Timeout,
	}
	if p.ServerSupport {
		p.Req.Header.Set("X-HTTPPING-REQUIRE", "TCPINFO")
	}

	resp, err := client.Do(p.Req)
	httpInfo.Domain = w.domain
	if w.remoteAddr != nil {
		httpInfo.Ip = w.remoteAddr.IP.String()
		httpInfo.Port = w.remoteAddr.Port
		httpInfo.DnsTimeMs = uint32(w.dnsTime.Milliseconds())
	}

	if err != nil {
		httpInfo.Error = err.Error()
		return err
	}
	httpInfo.ConnectTimeMs = uint32(w.tcpHandshake.Milliseconds())
	httpInfo.TLSHandshakeTimeMs = uint32(w.tlsHandshake.Milliseconds())
	httpInfo.TtfbMs = uint32(w.TTFB().Milliseconds())
	defer resp.Body.Close()
	httpInfo.Code = resp.StatusCode
	var done string
	if p.ServerSupport {
		done = resp.Header.Get("X-HTTPPING-TCPINFO")
	}
	defer resp.Body.Close()
	if done != "" && resp.ContentLength > 0 {
		err = dealWithServerTcpInfo(resp.Body, resp.ContentLength, &httpInfo.Server)
	} else if resp.ContentLength > 0 {
		err = readN(resp.Body, int(resp.ContentLength), p.BodyHasher)
	} else {
		err = readAll(resp.Body, p.BodyHasher)
	}
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		httpInfo.Error = err.Error()
		return err
	}

	tcpInfo, _, err := network.GetSockoptTCPInfo(w.d)
	if err != nil {
		httpInfo.Error = err.Error()
	} else {
		httpInfo.Client = *tcpInfo
	}

	if done != "" && resp.ContentLength != 0 {
		if httpInfo.Server.TotalPackets == 0 {
			httpInfo.Server.TotalPackets = uint32(w.count / 1460)
			if httpInfo.Server.TotalPackets == 0 {
				httpInfo.Server.TotalPackets = 1
			}
		}
		if httpInfo.Server.ReTransmitPackets != 0 {
			httpInfo.Loss = float32(httpInfo.Server.ReTransmitPackets) / float32(httpInfo.Server.TotalPackets) * 100.0
			httpInfo.ReTransmitPackets = httpInfo.Server.ReTransmitPackets
		}
	}
	return err
}

func Ping(req *http.Request, ping bool, srcAddr string) (*Info, error) {
	pinger := Pinger{
		Req:           req,
		SysPing:       ping,
		SrcAddr:       srcAddr,
		ServerSupport: false,
		BodyHasher:    nil,
	}
	return pinger.Ping()
}
