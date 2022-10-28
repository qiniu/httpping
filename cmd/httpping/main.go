package main

import (
	"flag"
	"fmt"
	"github.com/longbai/ping/network"
	"net/http"
)

func main() {
	url := flag.String("u", "www.baidu.com", "ping url")
	ping := flag.Bool("p", true, "with system ping command")
	local := flag.String("l", "", "local address")
	range_ := flag.String("r", "", "http range")
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

	info, err := network.HttpPing(req, *ping, *local)
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		return
	}

	fmt.Println(info.String())
}
