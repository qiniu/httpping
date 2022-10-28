package http

import (
	"fmt"
	"testing"
)
import "github.com/stretchr/testify/assert"

func TestHttp(t *testing.T) {
	h, err := HttpPingSimple("www.baidu.com")
	fmt.Println(h, err)
	assert.Nil(t, err)
	assert.NotNil(t, h)
}
