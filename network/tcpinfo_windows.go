//go:build windows

package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func GetSockoptTCPInfo(tcpConn *net.TCPConn) (*TCPInfo, interface{}, error) {
	tcpInfo, err := getConnInfo(tcpConn)
	if err != nil {
		return nil, nil, err
	}
	return tcpInfo.common(), &tcpInfo, nil
}

type MIB_TCP_STATE int32

type TCP_CONNECTION_OFFLOAD_STATE int32

type MIB_TCPROW2 struct {
	DwState        uint32
	DwLocalAddr    uint32
	DwLocalPort    uint32
	DwRemoteAddr   uint32
	DwRemotePort   uint32
	DwOwningPid    uint32
	DwOffloadState TCP_CONNECTION_OFFLOAD_STATE
}

type IN6_ADDR_U struct {
	Uchar  [16]byte
	Ushort [8]uint16
}

type IN6_ADDR struct {
	U IN6_ADDR_U
}

func (u *IN6_ADDR_U) GetByte() [16]byte {
	var ret [16]byte
	for i := 0; i < 16; i++ {
		ret[i] = u.Uchar[i]
	}
	return ret
}

type MIB_TCP6ROW2 struct {
	LocalAddr       IN6_ADDR
	DwLocalScopeId  uint32
	DwLocalPort     uint32
	RemoteAddr      IN6_ADDR
	DwRemoteScopeId uint32
	DwRemotePort    uint32
	State           MIB_TCP_STATE
	DwOwningPid     uint32
	DwOffloadState  TCP_CONNECTION_OFFLOAD_STATE
}

const TcpConnectionEstatsPath = 3

var (
	// Library
	modiphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")

	// Functions
	procGetPerTcpConnectionEStats  = modiphlpapi.NewProc("GetPerTcpConnectionEStats")
	procGetPerTcp6ConnectionEStats = modiphlpapi.NewProc("GetPerTcp6ConnectionEStats")
)

func getUintptrFromBool(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

func getConnInfo(tcpConn *net.TCPConn) (*TCPInfoWindows, error) {
	remote, _ := tcpConn.RemoteAddr().(*net.TCPAddr)
	local, _ := tcpConn.LocalAddr().(*net.TCPAddr)
	//tcp4
	row := &MIB_TCPROW2{
		DwState:        0,
		DwLocalAddr:    ip2int(local.IP),
		DwLocalPort:    uint32(local.Port),
		DwRemoteAddr:   ip2int(remote.IP),
		DwRemotePort:   uint32(remote.Port),
		DwOwningPid:    0,
		DwOffloadState: 0,
	}

	rod := TCPInfoWindows{}
	rodsize := unsafe.Sizeof(rod)

	r1, _, _ := syscall.Syscall12(procGetPerTcpConnectionEStats.Addr(),
		uintptr(unsafe.Pointer(&row)), TcpConnectionEstatsPath,
		uintptr(0), 0, 0,
		uintptr(0), 0, 0,
		uintptr(unsafe.Pointer(&rod)), 0, rodsize,
		0, 0)
	if r1 != 0 {
		errcode := syscall.Errno(r1)
		fmt.Println(errcode)
		if errcode != 0 {
			return nil, fmt.Errorf("syscall failed. errno=%d", errcode)
		}
	}
	//tcp6
	return &rod, nil
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		panic("no sane way to convert ipv6 into uint32")
	}
	return binary.BigEndian.Uint32(ip)
}
