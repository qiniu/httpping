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
	"time"

	h "github.com/qiniu/httpping/http"
)

func main() {
	url := flag.String("u", "www.baidu.com", "ping url")
	ping := flag.Bool("p", true, "with system ping command")
	local := flag.String("l", "", "local address")
	range_ := flag.String("r", "", "http range")
	server := flag.Bool("s", false, "server support tcpinfo return")
	hashStr := flag.String("hash", "", "body hash")
	ua := flag.String("ua", "", "user agent")
	redirect := flag.Bool("redirect", false, "enable redirect")
	timeout := flag.Int64("timeout", 10, "total timeout, seconds")
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
	if *ua != "" {
		req.Header.Set("User-Agent", *ua)
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
	p := h.Pinger{
		Req:           req,
		SysPing:       *ping,
		SrcAddr:       *local,
		ServerSupport: *server,
		BodyHasher:    hasher,
		Redirect:      *redirect,
		Timeout:       time.Duration(*timeout) * time.Second,
	}
	info, err := p.Ping()
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		return
	}

	fmt.Println(info.String())
}
