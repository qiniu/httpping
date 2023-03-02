package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"unsafe"

	"github.com/qiniu/httpping/network"
)

type contextKey struct {
	key string
}

var ConnContextKey = &contextKey{"http-conn"}

const (
	infoSize = int(unsafe.Sizeof(network.TCPInfo{}))
)

var DefaultContent = make([]byte, 2*1024*1024)

const MaxLength = 2 * 1024 * 1024

func writeBody(w http.ResponseWriter, sendSize int) (err error) {
	l := len(DefaultContent)
	left := sendSize
	var n int
	for {
		send := MinInt(l, left)
		n, err = w.Write(DefaultContent[:send])
		if err != nil {
			return
		}
		left -= n
		if left <= 0 {
			return
		}
	}
}

func MinInt(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func SaveConnInContext(ctx context.Context, c net.Conn) context.Context {
	if v, ok := c.(*net.TCPConn); ok {
		return context.WithValue(ctx, ConnContextKey, v)
	}
	return ctx
}

func GetConn(r *http.Request) *net.TCPConn {
	return r.Context().Value(ConnContextKey).(*net.TCPConn)
}

func getLength(r *http.Request) int {
	lengthStr := r.Header.Get("X-QN-QOT-LEN")
	length := len(DefaultContent)
	var err error
	if lengthStr != "" {
		length, err = strconv.Atoi(lengthStr)
		if err != nil {
			return -1
		}
		if length > MaxLength {
			length = MaxLength
		}
	}
	return length
}

func GetTcpInfo(r *http.Request) (*network.TCPInfo, error) {
	conn := GetConn(r)
	info, raw, err := network.GetSockoptTCPInfo(conn)
	if t, ok := raw.(*network.TCPInfoLinux); ok {
		realSent := getLength(r) - int(t.Tcpi_notsent_bytes)
		info.TotalPackets = uint32(realSent / 1460) // mss 1460, ignore http header, tcpinfo，some packets had sent are waiting ack, so it is not accurate
	}
	return info, err
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("hello"))
	})
	http.HandleFunc("/qn_download", HandleDownload)
	http.HandleFunc("/redirect", func(writer http.ResponseWriter, request *http.Request) {
		site := request.URL.Query().Get("q")
		writer.Header().Set("Location", site)
		writer.WriteHeader(301)
		print(site)
	})

	server := http.Server{
		Addr:        ":8082",
		ConnContext: SaveConnInContext,
	}
	server.ListenAndServe()
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-HTTPPING-TCPINFO", "DONE")
	tinfo := network.TCPInfo{}

	info, err := GetTcpInfo(r)
	if err == nil {
		tinfo = *info
	}
	fmt.Printf("%+v\n", tinfo, err)
	p := (*[infoSize]byte)(unsafe.Pointer(&tinfo))[:]
	fmt.Println(p)
	w.Write(p)
}

func HandleDownload(w http.ResponseWriter, r *http.Request) {
	length := getLength(r)
	if length <= 0 {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(length))
	tcpInfoReq := r.Header.Get("X-HTTPPING-REQUIRE")

	if tcpInfoReq == "TCPINFO" {
		w.Header().Set("X-HTTPPING-TCPINFO", "DONE")
		length -= infoSize
	}
	w.WriteHeader(http.StatusOK)
	writeBody(w, length)
	tinfo := network.TCPInfo{}
	if tcpInfoReq == "TCPINFO" {
		info, err := GetTcpInfo(r)
		if err == nil {
			tinfo = *info
		}
		p := (*[infoSize]byte)(unsafe.Pointer(&tinfo))[:]
		w.Write(p)
	}
}
