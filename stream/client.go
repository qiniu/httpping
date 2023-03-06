package stream

import (
	"errors"
	"net/http"
)

const (
	PktAudio = 0
	PktVideo = 1
)

type AVPacket struct {
	pktType  uint32
	pts      uint32
	keyframe bool
}

type TsSegment struct {
	url   string
	seqId uint64
}

type PAT struct {
	programs []PATProgram
}

type PATProgram struct {
	programNumber uint32
	programMapPid uint32
}

type PMT struct {
	pmtStreams []PMTStream
}

type PMTStream struct {
	elementaryPid uint32
	streamType    uint32
}

const (
	STREAM_TYPE_AUDIO_AAC  = 0x0f
	STREAM_TYPE_VIDEO_H264 = 0x1b
	STREAM_TYPE_VIDEO_HEVC = 0x24
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrInvalidTsPacket     = errors.New("invalid ts packet")
	ErrInvalidPESHeader    = errors.New("invalid pes header")
	ErrTryAgain            = errors.New("try again")
	ErrNotLiveM3u8File     = errors.New("not live m3u8 file")
)

var (
	ErrTcpConnectTimeout = 1001
	ErrInvalidHttpCode   = 1002
	ErrInternal          = 1003
)

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

type Client interface {
	Connect() (*StreamInfo, error)
	Read() (*AVPacket, error)
	Close()
}
