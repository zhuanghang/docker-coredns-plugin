package docker

import (
	"testing"
)

func Test1(t *testing.T) {
	p := NewPlugin()
	p.init()
	go p.handleEvent()
}