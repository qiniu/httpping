package stream

import (
	"context"
	"log"
	"time"
)

type Player struct {
	ch           chan AVPacket
	vqueue       []AVPacket
	aqueue       []AVPacket
	bufferTimeMs time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	info         *StreamInfo
}

func NewPlayer(playerBufferTimeMs uint32, info *StreamInfo) *Player {
	if playerBufferTimeMs > 30000 {
		playerBufferTimeMs = 30000
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Player{
		ctx:          ctx,
		cancel:       cancel,
		ch:           make(chan AVPacket, 256),
		vqueue:       make([]AVPacket, 0, 256),
		aqueue:       make([]AVPacket, 0, 256),
		bufferTimeMs: time.Duration(playerBufferTimeMs),
		info:         info,
	}
}

func (p *Player) Do() {
	var frameDuration time.Duration
	var audioFrameDuration time.Duration
	var lagTime time.Time
	var startTime time.Time

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

			log.Println("player cycle end")
			return

		case pkt := <-p.ch:
			if pkt.pktType == PktVideo {
				log.Println("video pkt pts=", time.Duration(pkt.pts)*time.Millisecond, len(p.vqueue))
				if !hasVideo {
					hasVideo = true
					p.info.FirstVideoPktTimeMs = uint32(time.Since(p.info.StartTime).Milliseconds())
					log.Println("receive first video=", time.Since(p.info.StartTime))
				}

				p.vqueue = append(p.vqueue, pkt)

				if !startPlay && len(p.vqueue) >= 60 {
					// estimated frame rate
					lastPts := int32(p.vqueue[0].pts)
					count := 0
					totalDuration := int32(0)

					for i := 1; i < len(p.vqueue); i++ {
						pkt := p.vqueue[i]
						if int32(pkt.pts) > lastPts && int32(pkt.pts)-lastPts < 100 {
							totalDuration += int32(pkt.pts) - lastPts
							count++
						}
						lastPts = int32(pkt.pts)
					}

					fps := float32(30)
					if totalDuration != 0 {
						fps = float32(count) / float32(totalDuration) * 1000
					}

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

						log.Println("fps=", fps, ",frame duration=", frameDuration,
							"buffer time=", p.bufferTimeMs*time.Millisecond)
					}
				}
			} else {
				//log.Println("audio pkt pts=", pkt.pts, len(p.aqueue))
				if !hasAudio {
					hasAudio = true
					p.info.FirstAudioPktTimeMs = uint32(time.Since(p.info.StartTime).Milliseconds())
					log.Println("receive first audio=", time.Since(p.info.StartTime))
				}

				p.aqueue = append(p.aqueue, pkt) //TODO:: support audio-only stream
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
				log.Println("rebuffer cost time=", time.Since(lagTime))
			}

			if rebuffer {
				break
			}

			if len(*queue) != 0 {
				*queue = (*queue)[1:]
			} else {
				// play lag occurs
				rebuffer = true
				p.info.TotalLagCount++
				lagTime = time.Now()
			}
		}
	}
}

func (p *Player) Close() {
	log.Println("player Close")
	if p.cancel != nil {
		p.cancel()
	}

	if p.ch != nil {
		close(p.ch)
	}
}
