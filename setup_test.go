package docker

import (
	"testing"
)

func Test1(t *testing.T) {
	//m := make(map[string]string)
	//m["123"] = "111"
	//a,b := m["1"]
	//println(a)
	//println(b)


	p := NewPlugin()
	p.init()
	go p.handleEvent()
}