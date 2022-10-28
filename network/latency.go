/*
	Copyright 2013-2014 Graham King
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
For full license details see <http://www.gnu.org/licenses/>.
*/

package network

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

func Latency(localAddr string, remoteHost string, port uint16) time.Duration {
	var wg sync.WaitGroup
	wg.Add(1)
	var receiveTime time.Time

	addrs, err := net.LookupHost(remoteHost)
	if err != nil {
		log.Fatalf("Error resolving %s. %s\n", remoteHost, err)
	}
	fmt.Println(addrs)
	remoteAddr := addrs[0]

	go func() {
		receiveTime = receiveSynAck(localAddr, remoteAddr)
		wg.Done()
	}()

	time.Sleep(1 * time.Millisecond)
	sendTime := sendSyn(localAddr, remoteAddr, port)

	wg.Wait()
	return receiveTime.Sub(sendTime)
}

func sendSyn(laddr, raddr string, port uint16) time.Time {

	packet := TCPHeader{
		Source:      0xaa47, // Random ephemeral port
		Destination: port,
		SeqNum:      rand.Uint32(),
		AckNum:      0,
		DataOffset:  5,      // 4 bits
		Reserved:    0,      // 3 bits
		ECN:         0,      // 3 bits
		Ctrl:        2,      // 6 bits (000010, SYN bit set)
		Window:      0xaaaa, // The amount of data that it is able to accept in bytes
		Checksum:    0,      // Kernel will set this if it's 0
		Urgent:      0,
		Options:     []TCPOption{},
	}

	data := packet.Marshal()
	packet.Checksum = Csum(data, to4byte(laddr), to4byte(raddr))

	data = packet.Marshal()

	//fmt.Printf("% x\n", data)

	conn, err := net.Dial("ip4:tcp", raddr)
	if err != nil {
		log.Fatalf("Dial: %s\n", err)
	}

	sendTime := time.Now()

	numWrote, err := conn.Write(data)
	if err != nil {
		log.Fatalf("Write: %s\n", err)
	}
	if numWrote != len(data) {
		log.Fatalf("Short write. Wrote %d/%d bytes\n", numWrote, len(data))
	}

	conn.Close()

	return sendTime
}

func to4byte(addr string) [4]byte {
	parts := strings.Split(addr, ".")
	b0, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Fatalf("to4byte: %s (latency works with IPv4 addresses only, but not IPv6!)\n", err)
	}
	b1, _ := strconv.Atoi(parts[1])
	b2, _ := strconv.Atoi(parts[2])
	b3, _ := strconv.Atoi(parts[3])
	return [4]byte{byte(b0), byte(b1), byte(b2), byte(b3)}
}

func receiveSynAck(localAddress, remoteAddress string) time.Time {
	netaddr, err := net.ResolveIPAddr("ip", localAddress)
	if err != nil {
		log.Fatalf("net.ResolveIPAddr: %s. %s\n", localAddress, netaddr)
	}
	conn, err := net.ListenIP("ip4:tcp", netaddr)
	if err != nil {
		log.Fatalf("ListenIP: %s\n", err)
	}
	var receiveTime time.Time
	for {
		buf := make([]byte, 1024)
		numRead, raddr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Fatalf("ReadFrom: %s\n", err)
		}
		if raddr.String() != remoteAddress {
			// this is not the packet we are looking for
			continue
		}
		receiveTime = time.Now()
		//fmt.Printf("Received: % x\n", buf[:numRead])
		tcp := NewTCPHeader(buf[:numRead])
		// Closed port gets RST, open port gets SYN ACK
		if tcp.HasFlag(RST) || (tcp.HasFlag(SYN) && tcp.HasFlag(ACK)) {
			break
		}
	}
	return receiveTime
}
