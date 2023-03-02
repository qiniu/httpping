package main

import (
	"encoding/json"
	"flag"
	"log"

	"github.com/qiniu/httpping/stream"
)

func main() {
	url := flag.String("u", "", "live stream url")
	playerBufferTimeMs := flag.Uint("player_buffer", 3000, "player buffer time")
	probeTimeSec := flag.Uint("probe_time", 60, "probe time")
	flag.Parse()

	prober := &stream.Prober{
		Url:                *url,
		PlayerBufferTimeMs: uint32(*playerBufferTimeMs),
		ProbeTimeSec:       uint32(*probeTimeSec),
	}

	info, err := prober.Do()
	if err != nil {
		log.Println("prober err:", err)
		return
	}

	data, _ := json.Marshal(info)
	log.Println("prober result:", string(data))
}
