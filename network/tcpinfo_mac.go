//go:build amd64 && darwin

package network

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const TCP_CONNECTION_INFO = 0x106
const IPPROTO_TCP = 6

// copy from netinet/tcp.h
type TCPInfo struct {
	Tcpistate               uint8 /* connection state */
	Tcpisnd_wscale          uint8 /* Window scale for send window */
	Tcpircv_wscale          uint8 /* Window scale for receive window */
	__pad1                  uint8
	Tcpioptions             uint32 /* TCP options supported */
	Tcpiflags               uint32 /* flags */
	Tcpirto                 uint32 /* retransmit timeout in ms */
	Tcpimaxseg              uint32 /* maximum segment size supported */
	Tcpisnd_ssthresh        uint32 /* slow start threshold in bytes */
	Tcpisnd_cwnd            uint32 /* send congestion window in bytes */
	Tcpisnd_wnd             uint32 /* send widnow in bytes */
	Tcpisnd_sbbytes         uint32 /* bytes in send socket buffer, including in-flight data */
	Tcpircv_wnd             uint32 /* receive window in bytes*/
	Tcpirttcur              uint32 /* most recent RTT */
	Rtt                     uint32
	Rttvar                  uint32
	Tcpi_tfo                uint32
	Tcpitxpackets           uint64
	Tcpitxbytes             uint64
	Tcpitxretransmitbytes   uint64
	Tcpirxpackets           uint64
	Tcpirxbytes             uint64
	Tcpirxoutoforderbytes   uint64
	Tcpitxretransmitpackets uint64
}

func GetsockoptTCPInfo(tcpConn *net.TCPConn) (*TCPInfo, error) {
	if tcpConn == nil {
		return nil, fmt.Errorf("tcp conn is nil")
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("error getting raw connection. err=%v", err)
	}

	tcpInfo := TCPInfo{}
	size := unsafe.Sizeof(tcpInfo)
	var errno syscall.Errno
	err = rawConn.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, IPPROTO_TCP, TCP_CONNECTION_INFO,
			uintptr(unsafe.Pointer(&tcpInfo)), uintptr(unsafe.Pointer(&size)), 0)
	})
	if err != nil {
		return nil, fmt.Errorf("rawconn control failed. err=%v", err)
	}

	if errno != 0 {
		return nil, fmt.Errorf("syscall failed. errno=%d", errno)
	}
	tcpInfo.Rtt *= 1000 // unify to linux
	tcpInfo.Tcpirttcur *= 1000
	tcpInfo.Rttvar *= 1000

	return &tcpInfo, nil
}
