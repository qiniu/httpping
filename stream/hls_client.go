package stream

import (
	"context"
	"github.com/grafov/m3u8"
	mhttp "github.com/qiniu/httpping/http"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type HlsClient struct {
	url           string
	secondM3u8Url string
	scheme        string
	host          string
	header        map[string]string
	timeout       time.Duration
	m3u8Ctx       context.Context
	m3u8Cancel    context.CancelFunc
	playlist      []TsSegment
	lastSeqId     int64
	mutex         sync.Mutex
	buffer        []byte
	pat           PAT
	pmt           PMT
}

func (c *HlsClient) Connect() (*StreamInfo, error) {
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

	resp, err := hc.Do(req)
	if err != nil {
		info.ErrCode = ErrTcpConnectTimeout
		return info, err
	}
	defer resp.Body.Close()

	info.init(tcp, resp)
	if resp.StatusCode != http.StatusOK {
		info.ErrCode = ErrInvalidHttpCode
		return info, nil
	}

	c.m3u8Ctx, c.m3u8Cancel = context.WithCancel(context.Background())
	go c.downloadM3u8()

	return info, err
}

func (c *HlsClient) Read() (*AVPacket, error) {
	if len(c.buffer) == 0 {
		var url string
		c.mutex.Lock()
		if len(c.playlist) != 0 {
			url = c.playlist[0].url
			c.playlist = c.playlist[1:]
		}
		c.mutex.Unlock()

		if url == "" {
			time.Sleep(time.Second)
			return nil, ErrTryAgain
		}

		req, err := newRequest(url, nil)
		if err != nil {
			return nil, err
		}

		hc := &http.Client{Timeout: c.timeout}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			//TODO: 统计错误状态码
			return nil, ErrTryAgain
		}

		c.buffer, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	}

	return c.demux()
}

func (c *HlsClient) Close() {
	if c.m3u8Cancel != nil {
		c.m3u8Cancel()
	}
	return
}

func (c *HlsClient) demux() (*AVPacket, error) {
	for len(c.buffer) >= 188 {
		data := c.buffer[:188]
		c.buffer = c.buffer[188:]

		if data[0] != 0x47 {
			return nil, ErrInvalidTsPacket
		}

		/*
		 * sync_byte                        8 bit
		 * transport_error_indicator        1 bit
		 * payload_unit_start_indicator     1 bit
		 * transport_priority               1 bit
		 * pid                              13 bit
		 * transport_scrambling_control     2 bit
		 * adaptation_field_control         2 bit
		 * continuity_count                 4 bit
		 */

		payloadUnitStartIndicator := (data[1] & 0x40) >> 6
		pid := (uint32(data[1]&0x1f) << 8) | uint32(data[2])
		adaptationFieldControl := (data[3] & 0x30) >> 4
		//continuityCount := (data[3] & 0x0f)

		if pid == 0x01 || pid == 0x02 || pid == 0x03 || pid == 0x11 || pid == 0x42 || pid == 0x1fff {
			/* ignore */
			continue
		}

		/*
		 * adaption_field_control：
		 * 0x00:    reserved for future use by ISO/IEC
		 * 0x01:    no adaption field, only payload
		 * 0x02:    only adaption field, no payload
		 * 0x03:    both adaption field and payload
		 */

		if adaptationFieldControl == 0x00 || adaptationFieldControl == 0x02 {
			continue
		}

		data = data[4:]

		if adaptationFieldControl == 0x03 {
			c.decodeAdaptationFiled(&data)
		}

		// decode PAT
		if pid == 0x0 {
			if payloadUnitStartIndicator != 0 {
				data = data[1:]
			}

			c.decodePAT(data)
			continue
		}

		var pmtFound bool
		for _, program := range c.pat.programs {
			if program.programMapPid == pid {
				pmtFound = true
				break
			}
		}

		// decode PMT
		if pmtFound {
			if payloadUnitStartIndicator != 0 {
				data = data[1:]
			}

			c.decodePMT(data)
			continue
		}

		return c.decodeStream(data, pid, payloadUnitStartIndicator != 0)
	}

	c.buffer = nil
	return nil, ErrTryAgain
}

func (c *HlsClient) decodeStream(data []byte, pid uint32, payloadStart bool) (*AVPacket, error) {
	var foundStream bool
	var streamType uint32
	for _, s := range c.pmt.pmtStreams {
		if pid == s.elementaryPid {
			foundStream = true
			streamType = s.streamType
			break
		}
	}

	if !foundStream || !payloadStart {
		return nil, ErrTryAgain
	}

	var pktType uint32
	if streamType == STREAM_TYPE_VIDEO_H264 || streamType == STREAM_TYPE_VIDEO_HEVC {
		pktType = PktVideo
	} else {
		pktType = PktAudio
	}

	pkt, err := c.decodePES(data, pktType)
	if err != nil {
		return nil, err
	}

	return pkt, nil
}

func (c *HlsClient) decodePES(data []byte, pktType uint32) (*AVPacket, error) {
	/* packet_start_code_prefix                     24 bslbf */
	packetStartCodePrefix := (uint32(data[0]) << 16) |
		(uint32(data[1]) << 8) |
		uint32(data[2])

	if packetStartCodePrefix != 0x000001 {
		return nil, ErrInvalidPESHeader
	}

	data = data[3:]
	streamId := uint32(data[0])
	data = data[3:]

	if streamId != 188 &&
		streamId != 190 &&
		streamId != 191 &&
		streamId != 240 &&
		streamId != 241 &&
		streamId != 255 &&
		streamId != 242 &&
		streamId != 248 {

		if data[0]&0xc0 != 0x80 {
			return nil, ErrInvalidPESHeader
		}

		data = data[1:]

		/*
		 * PTS_DTS_flags                            2  bslbf
		 * ESCR_flag                                1  bslbf
		 * ES_rate_flag                             1  bslbf
		 * DSM_trick_mode_flag                      1  bslbf
		 * additional_copy_info_flag                1  bslbf
		 * PES_CRC_flag                             1  bslbf
		 * PES_extension_flag                       1  bslbf
		 */

		PTS_DTS_flags := (data[0] & 0xc0) >> 6
		//ESCR_flag := (data[0] & 0x20) >> 5
		//ES_rate_flag := (data[0] & 0x10) >> 4
		//DSM_trick_mode_flag := (data[0] & 0x08) >> 3
		//additional_copy_info_flag := (data[0] & 0x04) >> 2
		//PES_CRC_flag := (data[0] & 0x02) >> 1
		//PES_extension_flag := (data[0] & 0x01)

		/* PES_header_data_length                    8  uimsbf */
		data = data[2:]

		if PTS_DTS_flags == 2 {
			/*
			 * '0010'                                 4  bslbf
			 * PTS [32..30]                           3  bslbf
			 * marker_bit                             1  bslbf
			 * PTS [29..15]                           15 bslbf
			 * marker_bit                             1  bslbf
			 * PTS [14..0]                            15 bslbf
			 * marker_bit                             1  bslbf
			 */

			if (data[0]&0xf0)>>4 != 2 {
				return nil, ErrInvalidPESHeader
			}

			pts := (uint32((data[0]>>1)&0x07) << 30) |
				(uint32(data[1]) << 22) |
				((uint32(data[2]>>1) & 0x7f) << 15) |
				(uint32(data[3]) << 7) |
				uint32(data[4]>>1)

			pts /= 90

			return &AVPacket{
				pts:      pts,
				pktType:  pktType,
				keyframe: true,
			}, nil

		} else if PTS_DTS_flags == 3 {
			/*
			 * '0011'                               4  bslbf
			 * PTS [32..30]                         3  bslbf
			 * marker_bit                           1  bslbf
			 * PTS [29..15]                         15 bslbf
			 * marker_bit                           1  bslbf
			 * PTS [14..0]                          15 bslbf
			 * marker_bit                           1  bslbf
			 */

			if (data[0]&0xf0)>>4 != 3 {
				return nil, ErrInvalidPESHeader
			}

			pts := (uint32((data[0]>>1)&0x07) << 30) |
				(uint32(data[1]) << 22) |
				((uint32(data[2]>>1) & 0x7f) << 15) |
				(uint32(data[3]) << 7) |
				uint32(data[4]>>1)

			pts /= 90

			return &AVPacket{
				pts:      pts,
				pktType:  pktType,
				keyframe: true,
			}, nil
		}
	}

	return nil, ErrTryAgain
}

func (c *HlsClient) decodeAdaptationFiled(data *[]byte) {
	adaptationFieldLen := (*data)[0]
	*data = (*data)[1:]

	if adaptationFieldLen > 0 {
		*data = (*data)[adaptationFieldLen:]
		return
	}
}

func (c *HlsClient) decodePAT(data []byte) {
	var pat PAT
	sectionLength := int32(data[1]&0x0f)<<8 | int32(data[2])
	data = data[8:]

	for i := int32(0); i < sectionLength-9 && len(data) != 0; i += 4 {
		programNum := uint32(data[0])<<8 | uint32(data[1])
		if programNum != 0x00 {
			programMapPid := (uint32(data[2])<<8 | uint32(data[3])) & 0x1fff
			pat.programs = append(pat.programs, PATProgram{
				programNumber: programNum,
				programMapPid: programMapPid})
		}
		data = data[4:]
	}

	c.pat = pat
}

func (c *HlsClient) decodePMT(data []byte) {
	var pmt PMT
	sectionLength := (int32(data[1]&0x0f) << 8) | int32(data[2])
	programInfoLength := (int32(data[10]&0x0f) << 8) | int32(data[11])
	data = data[12+programInfoLength:]

	for i := int32(0); i < sectionLength-9-5 && len(data) != 0; i += 5 {
		stream := PMTStream{}
		stream.streamType = uint32(data[0])
		stream.elementaryPid = ((uint32(data[1]) << 8) | uint32(data[2])) & 0x1fff
		esInfoLength := uint32(data[3]&0x0f)<<8 | uint32(data[4])
		data = data[5+esInfoLength:]
		pmt.pmtStreams = append(pmt.pmtStreams, stream)
	}

	if len(pmt.pmtStreams) != 0 {
		c.pmt = pmt
	}
}

func (c *HlsClient) doRequest(url string) (time.Duration, error) {
	req, err := newRequest(url, nil)
	if err != nil {
		return 0, err
	}

	hc := &http.Client{}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	interval, err := c.decodeM3u8(resp.Body)
	if err != nil {
		return 0, err
	}
	return interval, nil
}

func (c *HlsClient) downloadM3u8() {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.m3u8Ctx.Done():
			return

		case <-ticker.C:
			url := c.url
			if c.secondM3u8Url != "" {
				url = c.secondM3u8Url
			}
			interval, err := c.doRequest(url)
			if err != nil {
				ticker.Reset(time.Second)
				break
			}
			ticker.Reset(interval)
		}
	}
}

func (c *HlsClient) decodeM3u8(r io.Reader) (time.Duration, error) {
	playlist, mtype, err := m3u8.DecodeFrom(r, true)
	if err != nil {
		return time.Second, err
	}

	if mtype == m3u8.MASTER {
		masterPlaylist := playlist.(*m3u8.MasterPlaylist)
		c.secondM3u8Url = masterPlaylist.Variants[0].URI
		log.Println("second m3u8 file url=", c.secondM3u8Url)
		return time.Millisecond, nil
	}

	mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
	if mediaPlaylist.Closed {
		return time.Second, ErrNotLiveM3u8File
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, segment := range mediaPlaylist.Segments {
		if segment == nil || int64(segment.SeqId) <= c.lastSeqId {
			break
		}

		uri := segment.URI
		if !(strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")) {
			if strings.HasPrefix(uri, "/") {
				uri = c.scheme + "://" + c.host + uri
			} else {
				uri = c.scheme + "://" + c.host + "/" + uri
			}
			c.lastSeqId = int64(segment.SeqId)
		}

		log.Println("new ts url=", uri)
		c.playlist = append(c.playlist, TsSegment{
			url:   uri,
			seqId: segment.SeqId,
		})

	}

	//time.Duration(mediaPlaylist.TargetDuration/2) * time.Second,
	return time.Second, nil
}
