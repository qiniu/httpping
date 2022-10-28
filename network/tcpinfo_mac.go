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

type TCPInfoMac struct {
	Tcpi_state               uint8 /* connection state */
	Tcpi_snd_wscale          uint8 /* Window scale for send window */
	Tcpi_rcv_wscale          uint8 /* Window scale for receive window */
	__pad1                   uint8
	Tcpi_options             uint32 /* TCP options supported */
	Tcpi_flags               uint32 /* flags */
	Tcpi_rto                 uint32 /* retransmit timeout in ms */
	Tcpi_maxseg              uint32 /* maximum segment size supported */
	Tcpi_snd_ssthresh        uint32 /* slow start threshold in bytes */
	Tcpi_snd_cwnd            uint32 /* send congestion window in bytes */
	Tcpi_snd_wnd             uint32 /* send widnow in bytes */
	Tcpi_snd_sbbytes         uint32 /* bytes in send socket buffer, including in-flight data */
	Tcpi_rcv_wnd             uint32 /* receive window in bytes*/
	Tcpi_rttcur              uint32 /* most recent RTT in ms */
	Tcpi_srtt                uint32 /* average RTT in ms */
	Tcpi_rttvar              uint32 /* RTT variance */
	Tcpi__tfo                uint32
	Tcpi_txpackets           uint64
	Tcpi_txbytes             uint64
	Tcpi_txretransmitbytes   uint64
	Tcpi_rxpackets           uint64
	Tcpi_rxbytes             uint64
	Tcpi_rxoutoforderbytes   uint64
	Tcpi_txretransmitpackets uint64
}

func (t *TCPInfoMac) common() *TCPInfo {
	var tinfo TCPInfo
	tinfo.ReTransmitPackets = uint32(t.Tcpi_txretransmitpackets)
	tinfo.RttMs = t.Tcpi_srtt
	return &tinfo
}

func GetSockoptTCPInfo(tcpConn *net.TCPConn) (*TCPInfo, error) {
	if tcpConn == nil {
		return nil, fmt.Errorf("tcp conn is nil")
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("error getting raw connection. err=%v", err)
	}

	tcpInfo := TCPInfoMac{}
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

	return tcpInfo.common(), nil
}
