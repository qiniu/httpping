package sysping

import (
	"fmt"
	"testing"
)

func TestMacStats1(t *testing.T) {
	s := "16 packets transmitted, 15 packets received, 6.2% packet loss"
	result := matchAsMap(statsLine1, s)
	fmt.Println(result)
	s = "PING www.a.shifen.com (180.101.49.14): 56 data bytes"
	//s = "PING 127.0.0.1 (127.0.0.1): 56 data bytes"
	result = matchAsMap(headerRxAlt, s)
	fmt.Println(result)
}
