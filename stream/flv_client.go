package stream

import (
	"log"
	"net/http"
	"time"

	mhttp "github.com/qiniu/httpping/http"
	"github.com/yutopp/go-flv"
	"github.com/yutopp/go-flv/tag"
)

type FlvClient struct {
	url      string
	header   map[string]string
	timeout  time.Duration
	response *http.Response
	decoder  *flv.Decoder
}

func (c *FlvClient) Connect() (*StreamInfo, error) {
	info := &StreamInfo{StartTime: time.Now()}
	req, err := newRequest(c.url, nil)
	if err != nil {
		return info, err
	}

	tcp := &mhttp.TcpWrapper{}
	hc := &http.Client{
		Transport: &http.Transport{DialContext: tcp.Dial, DialTLSContext: tcp.DialTLS},
		Timeout:   c.timeout,
	}

	c.response, err = hc.Do(req)
	if err != nil {
		info.ErrCode = ErrTcpConnectTimeout
		return info, err
	}

	info.init(tcp, c.response)
	if c.response.StatusCode != http.StatusOK {
		info.ErrCode = ErrInvalidHttpCode
		return info, nil
	}

	c.decoder, err = flv.NewDecoder(c.response.Body)
	if err != nil {
		return info, err
	}

	return info, nil
}

func (c *FlvClient) Read() (*AVPacket, error) {
	flvTag := tag.FlvTag{}
	defer flvTag.Close()

	err := c.decoder.Decode(&flvTag)
	if err != nil {
		log.Println("invalid tag:", err)
		return nil, ErrTryAgain
	}

	if flvTag.TagType == tag.TagTypeVideo {
		videoData := (flvTag.Data).(*tag.VideoData)
		pts := int32(flvTag.Timestamp) + videoData.CompositionTime
		keyframe := videoData.FrameType == tag.FrameTypeKeyFrame

		return &AVPacket{
			pts:      uint32(pts),
			pktType:  PktVideo,
			keyframe: keyframe,
		}, nil
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

	return nil, ErrTryAgain
}

func (c *FlvClient) Close() {
	if c.response != nil {
		c.response.Body.Close()
	}
}
