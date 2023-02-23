package main

import (
	"encoding/json"
	"log"

	"github.com/qiniu/httpping/stream"
)

//122.228.252.90
//internal-player.cloudvdn.com
func main() {
	prober := &stream.Prober{
		Url:                "http://internal-player.cloudvdn.com/.i/anp0ZXN0MTI6anp0ZXN0MTIvdGVzdDAx.flv?role=portalio&cdn-src-referer=vdn&domain=internal-player.cloudvdn.com",
		PlayerBufferTimeMs: 0,
		ProbeTimeSec:       60,
	}

	info, err := prober.Do()
	if err != nil {
		log.Println("prober err:", err)
		return
	}

	data, _ := json.Marshal(info)
	log.Println("prober result:", string(data))
}
