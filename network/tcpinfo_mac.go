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
