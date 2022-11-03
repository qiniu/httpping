package main

import (
	"crypto/md5"
	"crypto/sha1"
	"flag"
	"fmt"
	"hash"
	"hash/crc32"
	"net/http"
	"strings"

	h "github.com/qiniu/httpping/http"
)

func main() {
	url := flag.String("u", "www.baidu.com", "ping url")
	ping := flag.Bool("p", true, "with system ping command")
	local := flag.String("l", "", "local address")
	range_ := flag.String("r", "", "http range")
	server := flag.Bool("s", false, "server support tcpinfo return")
	hashStr := flag.String("hash", "", "body hash")
	flag.Parse()

	req, err := http.NewRequest(http.MethodGet, *url, nil)
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		return
	}
	if *range_ != "" {
		req.Header.Set("Range", "bytes="+*range_)
	}
	var hasher hash.Hash
	switch strings.ToLower(*hashStr) {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "crc":
		hasher = crc32.NewIEEE()
	}
	info, err := h.HttpPingServerInfo(req, *ping, *local, *server, hasher)
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		return
	}

	fmt.Println(info.String())
}
