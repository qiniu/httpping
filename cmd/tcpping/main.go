package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/longbai/ping/network"
)

var (
	ifaceParam   = flag.String("i", "", "Interface (e.g. eth0, wlan1, etc)")
	helpParam    = flag.Bool("h", false, "Print help")
	portParam    = flag.Int("p", 80, "Port to test against (default 80)")
	autoParam    = flag.Bool("a", false, "Measure latency to several well known addresses")
	defaultHosts = map[string]string{
		// Busiest sites on the Internet, according to Wolfram Alpha
		"Google":   "google.com",
		"Facebook": "facebook.com",
		"Baidu":    "baidu.com",

		// Various locations, thanks Linode
		"West Coast, USA": "speedtest.fremont.linode.com",
		"East Coast, USA": "speedtest.newark.linode.com",
		"London, UK":      "speedtest.london.linode.com",
		"Tokyo, JP":       "speedtest.tokyo.linode.com",

		// Other continents
		"New Zealand":  "nzdsl.co.nz",
		"South Africa": "speedtest.mybroadband.co.za",
	}
)

func autoTest(localAddr string, port uint16) {
	for name, host := range defaultHosts {
		fmt.Printf("%15s: %v\n", name, network.Latency(localAddr, host, port))
	}
}

func main() {
	flag.Parse()

	if *helpParam {
		printHelp()
		os.Exit(1)
	}

	iface := *ifaceParam
	if iface == "" {
		iface = network.ChooseInterface()
		if iface == "" {
			fmt.Println("Could not decide which net interface to use.")
			fmt.Println("Specify it with -i <iface> param")
			os.Exit(1)
		}
	}

	localAddr := network.InterfaceAddress(iface)
	laddr := strings.Split(localAddr.String(), "/")[0] // Clean addresses like 192.168.1.30/24

	port := uint16(*portParam)
	if *autoParam {
		autoTest(laddr, port)
		return
	}

	if len(flag.Args()) == 0 {
		fmt.Println("Missing remote address")
		printHelp()
		os.Exit(1)
	}

	remoteHost := flag.Arg(0)
	fmt.Println("Measuring round-trip latency from", laddr, "to", remoteHost, "on port", port)
	fmt.Printf("Latency: %v\n", network.Latency(laddr, remoteHost, port))
}

func printHelp() {
	help := `
	USAGE: latency [-h] [-a] [-i iface] [-p port] <remote>
	Where 'remote' is an ip address or host name.
	Default port is 80
	-h: Help
	-a: Run auto test against several well known sites
	`
	fmt.Println(help)
}
