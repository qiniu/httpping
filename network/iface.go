package network

import (
	"log"
	"net"
	"strings"
)

func ChooseInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Fatalf("net.Interfaces: %s", err)
	}
	for _, iface := range interfaces {
		// Skip loopback
		if strings.HasPrefix(iface.Name, "lo") {
			continue
		}
		addrs, err := iface.Addrs()
		// Skip if error getting addresses
		if err != nil {
			log.Println("Error get addresses for interfaces %s. %s", iface.Name, err)
			continue
		}

		if len(addrs) > 0 {
			// This one will do
			return iface.Name
		}
	}

	return ""
}

func InterfaceAddress(ifaceName string) net.Addr {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		log.Fatalf("net.InterfaceByName for %s. %s", ifaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		log.Fatalf("iface.Addrs: %s", err)
	}
	var addr net.Addr
	for _, a := range addrs {
		if !strings.Contains(a.String(), ":") {
			addr = a
			break
		}
	}
	return addr
}
