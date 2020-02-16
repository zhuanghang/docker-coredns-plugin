package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"net"
	"testing"
)

func Test1(t *testing.T) {
	m := make(map[string]net.IP)
	m["123"] = net.IPv4(127,0,0,1)
	m["2"] = nil
	ip, exist := m["2"]
	println("ip: ", ip)
	println("exist: ", exist)

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:10], container.Image)
		json1, _ := cli.ContainerInspect(ctx, container.ID)
		str, _ := json.Marshal(json1)
		fmt.Println(string(str))
	}

	events, _ := cli.Events(ctx, types.EventsOptions {})
	for x := range events{
		fmt.Println("123")
		str, _ := json.Marshal(x)
		fmt.Println(string(str))
	}
}