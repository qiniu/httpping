package stream

import (
	"github.com/qiniu/httpping/network"
	"net/http"
	"net/url"
	"path"
	"time"

	mhttp "github.com/qiniu/httpping/http"
)

type Prober struct {
	Url                string
	PlayerBufferTimeMs uint32
	ProbeTimeSec       uint32
	Header             map[string]string
}

type StreamInfo struct {
	StartTime time.Time

	IsConnected         bool
	ErrCode             int
	DnsTimeMs           uint32
	TcpConnectTimeMs    uint32
	TLSHandshakeTimeMs  uint32
	TtfbMs              uint32
	FirstVideoPktTimeMs uint32
	FirstAudioPktTimeMs uint32
	TotalLagTimeMs      uint32
	TotalLagCount       uint32
	VideoFps            float32
	LagRate             float32
	HttpCode            int
	RemoteAddr          string
	LocalAddr           string
	TcpInfo             network.TCPInfo
}

func (info *StreamInfo) init(tcp *mhttp.TcpWrapper, resp *http.Response) {
	info.IsConnected = true
	info.DnsTimeMs = uint32(tcp.DnsTime().Milliseconds())
	info.TcpConnectTimeMs = uint32(tcp.TcpHandshake().Milliseconds())
	info.TLSHandshakeTimeMs = uint32(tcp.TlsHandshake().Milliseconds())
	info.TtfbMs = uint32(tcp.TTFB().Milliseconds())
	info.RemoteAddr = tcp.RemoteAddr().String()
	info.LocalAddr = tcp.LocalAddr().String()
	info.HttpCode = resp.StatusCode
	tinfo, err := tcp.CommonInfo()
	if err == nil {
		info.TcpInfo = *tinfo
	}
}

func (p *Prober) Do() (*StreamInfo, error) {
	u, err := url.Parse(p.Url)
	if err != nil {
		return nil, err
	}

	var client Client

	switch u.Scheme {
	case "http", "https":
		ext := path.Ext(u.Path)
		if ext == ".flv" {
			client = &FlvClient{url: p.Url}
		} else if ext == ".m3u8" {
			client = &HlsClient{url: p.Url, scheme: u.Scheme, host: u.Host, lastSeqId: -1}
		} else {
			return nil, ErrUnsupportedProtocol
		}

	default:
		return nil, ErrUnsupportedProtocol
	}

	info, err := p.do(client)

	return info, err
}

func (p *Prober) do(client Client) (*StreamInfo, error) {
	info, err := client.Connect()
	if err != nil {
		return info, err
	}
	defer client.Close()

	player := NewPlayer(p.PlayerBufferTimeMs, info)
	go player.Do()
	defer player.Close()

	timer := time.NewTimer(time.Duration(p.ProbeTimeSec) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return player.info, nil
		default:
		}

		pkt, err := client.Read()
		if err != nil {
			if err == ErrTryAgain {
				continue
			}

			return player.info, err
		}

		player.ch <- *pkt
	}

	return player.info, nil
}
