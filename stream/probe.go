package stream

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	mhttp "github.com/qiniu/httpping/http"
	"github.com/yutopp/go-flv"
	"github.com/yutopp/go-flv/tag"
)

type Prober struct {
	Url                string
	PlayerBufferTimeMs uint32
	ProbeTimeSec       uint32
	Header             map[string]string
}

type StreamInfo struct {
	StartTime time.Time

	ConnectOk           bool
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
}

func (info *StreamInfo) init(tcp *mhttp.TcpWrapper, resp *http.Response) {
	info.DnsTimeMs = uint32(tcp.DnsTime().Milliseconds())
	info.TcpConnectTimeMs = uint32(tcp.TcpHandshake().Milliseconds())
	info.TLSHandshakeTimeMs = uint32(tcp.TlsHandshake().Milliseconds())
	info.TtfbMs = uint32(tcp.TTFB().Milliseconds())
	info.RemoteAddr = tcp.RemoteAddr().String()
	info.LocalAddr = tcp.LocalAddr().String()
	info.HttpCode = resp.StatusCode
}

type AVPacket struct {
	pktType  uint32
	pts      uint32
	keyframe bool
}

type Player struct {
	ch           chan AVPacket
	vqueue       []AVPacket
	aqueue       []AVPacket
	bufferTimeMs time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	info         *StreamInfo
}

type Client interface {
	Connect() (*StreamInfo, error)
	Read() (*AVPacket, error)
	Close()
}

type FlvClient struct {
	url      string
	header   map[string]string
	response *http.Response
	decoder  *flv.Decoder
}

func (c *FlvClient) Connect() (*StreamInfo, error) {
	info := &StreamInfo{StartTime: time.Now()}
	req, err := newRequest(c.url, nil)
	if err != nil {
		return nil, err
	}

	tcp := &mhttp.TcpWrapper{}
	hc := &http.Client{
		Transport: &http.Transport{DialContext: tcp.Dial, DialTLSContext: tcp.DialTLS},
	}

	c.response, err = hc.Do(req)
	if err != nil {
		return nil, err
	}

	info.init(tcp, c.response)
	if c.response.StatusCode != 200 {
		return info, nil
	}

	c.decoder, err = flv.NewDecoder(c.response.Body)
	if err != nil {
		return info, err
	}

	return info, nil
}

func (c *FlvClient) Read() (*AVPacket, error) {
	var flvTag tag.FlvTag
	err := c.decoder.Decode(&flvTag)
	if err != nil {
		log.Println("invalid tag:", err)
		return nil, nil
	}

	if flvTag.TagType == tag.TagTypeVideo {
		videoData := (flvTag.Data).(*tag.VideoData)
		pts := flvTag.Timestamp + uint32(videoData.CompositionTime)
		keyframe := videoData.FrameType == tag.FrameTypeKeyFrame
		if videoData.AVCPacketType == tag.AVCPacketTypeNALU {
			return &AVPacket{
				pts:      pts,
				pktType:  PktVideo,
				keyframe: keyframe,
			}, nil
		}
	} else if flvTag.TagType == tag.TagTypeAudio {
		pts := flvTag.Timestamp
		audioData := (flvTag.Data).(*tag.AudioData)
		if audioData.AACPacketType == tag.AACPacketTypeRaw {
			return &AVPacket{
				pts:      pts,
				pktType:  PktAudio,
				keyframe: false,
			}, nil
		}
	}

	return nil, nil
}

func (c *FlvClient) Close() {
	if c.response != nil {
		c.response.Body.Close()
	}
}

type HlsClient struct {
}

func (c *HlsClient) Connect() (*StreamInfo, error) {
	//TODO:
	return nil, nil
}

func (c *HlsClient) Read() (*AVPacket, error) {
	//TODO:
	return nil, nil
}

func (c *HlsClient) Close() {
	//TODO:
	return
}

type RtmpClient struct {
	//TODO:
}

func (c *RtmpClient) Connect() (*StreamInfo, error) {
	//TODO:
	return nil, nil
}

func (c *RtmpClient) Read() (*AVPacket, error) {
	//TODO:
	return nil, nil
}

func (c *RtmpClient) Close() {
	//TODO:
	return
}

const (
	PktAudio = 1
	PktVideo = 2
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
)

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
			client = &HlsClient{}
		} else {
			return nil, ErrUnsupportedProtocol
		}

	case "rtmp":
		client = &RtmpClient{}

	default:
		return nil, ErrUnsupportedProtocol
	}

	info, err := p.do(client)

	time.Sleep(1 * time.Second)
	return info, err
}

func newRequest(url string, header map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}

	return req, nil
}

func (p *Prober) do(client Client) (*StreamInfo, error) {
	info, err := client.Connect()
	if err != nil {
		return info, err
	}
	defer client.Close()

	player := NewPlayer(p.PlayerBufferTimeMs, info)
	defer player.Close()
	go player.Do()

	timer := time.NewTimer(time.Duration(p.ProbeTimeSec) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			break
		default:
		}

		pkt, err := client.Read()
		if err != nil {
			//TODO:
			log.Println("yxl >>> break", err)
			return player.info, err
		}

		if pkt != nil {
			player.ch <- *pkt
		}
	}

	return player.info, nil
}

func NewPlayer(playerBufferTimeMs uint32, info *StreamInfo) *Player {
	if playerBufferTimeMs > 30000 {
		playerBufferTimeMs = 30000
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Player{
		ctx:          ctx,
		cancel:       cancel,
		ch:           make(chan AVPacket, 32),
		vqueue:       make([]AVPacket, 0, 128),
		aqueue:       make([]AVPacket, 0, 128),
		bufferTimeMs: time.Duration(playerBufferTimeMs),
		info:         info,
	}
}

func (p *Player) Do() {
	var frameDuration time.Duration
	var audioFrameDuration time.Duration
	var lagTime time.Time
	var startTime time.Time

	firstVideo := true
	firstAudio := true
	startPlay := false
	hasVideo := false
	hasAudio := false
	rebuffer := false

	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			if !startPlay {
				return
			}

			if rebuffer {
				p.info.TotalLagTimeMs += uint32(time.Since(lagTime).Milliseconds())
			}

			totalPlayTimeMs := float32(time.Since(startTime).Milliseconds())
			p.info.LagRate = float32(p.info.TotalLagTimeMs) / totalPlayTimeMs

			log.Println("player cycle over", p.info.TotalLagTimeMs, totalPlayTimeMs, p.info.LagRate)
			return

		case pkt := <-p.ch:
			if pkt.pktType == PktVideo {
				log.Println("video pkt pts=", pkt.pts, " ", len(p.vqueue))
				hasVideo = true
				if firstVideo {
					p.info.FirstVideoPktTimeMs = uint32(time.Since(p.info.StartTime).Milliseconds())
					firstVideo = false
					log.Println("receive first video=", time.Since(p.info.StartTime))
				}

				p.vqueue = append(p.vqueue, pkt)

				// estimated frame rate
				if !startPlay && len(p.vqueue) >= 60 {
					startPts := uint32(0)
					lastPts := uint32(0)
					count := 0

					for _, pkt := range p.vqueue {
						if pkt.pts-lastPts >= 200 || lastPts > pkt.pts {
							startPts = pkt.pts
							count = 0
						}

						lastPts = pkt.pts
						count++

						if count >= 30 {
							fps := float32(count-1) / float32(lastPts-startPts) * 1000
							p.info.VideoFps = fps
							frameDuration = time.Duration(1000000.0/fps) * time.Microsecond
							bufferTime := time.Duration(len(p.vqueue)) * frameDuration

							if bufferTime >= p.bufferTimeMs*time.Millisecond {
								startPlay = true
								startTime = time.Now()
								ticker.Reset(frameDuration)

								if p.bufferTimeMs != 0 {
									pktNum := uint32(p.bufferTimeMs * time.Millisecond / frameDuration)
									p.vqueue = p.vqueue[:pktNum]
								}

								log.Println("video fps=", fps, ",frame duration=", frameDuration,
									"buffer time=", p.bufferTimeMs*time.Millisecond)
							}

							break
						}
					}
				}

			} else {
				log.Println("audio pkt pts=", pkt.pts, len(p.aqueue))
				hasAudio = true
				if firstAudio {
					p.info.FirstAudioPktTimeMs = uint32(time.Since(p.info.StartTime).Milliseconds())
					firstAudio = false
					log.Println("receive first audio=", time.Since(p.info.StartTime))
				}

				p.aqueue = append(p.aqueue, pkt) //FIXME: support audio-only stream
			}

		case <-ticker.C:
			if !startPlay {
				break
			}

			var queue *[]AVPacket
			var duratuon time.Duration

			if hasVideo {
				queue = &p.vqueue
				duratuon = frameDuration
				p.aqueue = p.aqueue[0:0]
			} else if hasAudio {
				queue = &p.aqueue
				duratuon = audioFrameDuration
			}

			bufferTime := time.Duration(len(*queue)) * duratuon
			if rebuffer && bufferTime >= p.bufferTimeMs*time.Millisecond {
				rebuffer = false
				p.info.TotalLagTimeMs += uint32(time.Since(lagTime).Milliseconds())
				log.Println("yxl >>>> rebuffer use time=", time.Since(lagTime))
			}

			if rebuffer {
				break
			}

			if len(*queue) != 0 {
				log.Println("yxl >>>> before len:", len(*queue))
				*queue = (*queue)[1:]
				log.Println("yxl >>>> after len:", len(*queue))
			} else {
				rebuffer = true
				p.info.TotalLagCount++
				lagTime = time.Now()
				log.Println("yxl >>>> laging")
			}
		}
	}
}

func (p *Player) Close() {
	if p.cancel != nil {
		p.cancel()
	}

	if p.ch != nil {
		close(p.ch)
	}
}
