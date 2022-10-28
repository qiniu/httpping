package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPingResult(t *testing.T) {
	s := "16 packets transmitted, 15 packets received, 6.2% packet loss"
	result := matchAsMap(statsLine1, s)
	assert.NotEmpty(t, result)
	s = "PING www.a.shifen.com (180.101.49.14): 56 data bytes"
	result = matchAsMap(headerRxAlt, s)
	assert.NotEmpty(t, result)
	s = "PING www.a.shifen.com (180.101.49.14) from 192.168.31.111: 56 data bytes"
	result = matchAsMap(headerRxAlt, s)
	assert.NotEmpty(t, result)
	s = "PING www.a.shifen.com (110.242.68.3) from 162.219.87.156 : 56(84) bytes of data."
	result = matchAsMap(headerRx, s)
	assert.NotEmpty(t, result)
	s = "PING www.a.shifen.com (110.242.68.4) 56(84) bytes of data."
	result = matchAsMap(headerRx, s)
	assert.NotEmpty(t, result)
}
