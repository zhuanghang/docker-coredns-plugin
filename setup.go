package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/miekg/dns"
	"net"
)

var pluginName = "docker"

// Define log to be a logger with the plugin name in it. This way we can just use log.Info and
// friends to log.
var log = clog.NewWithPlugin(pluginName)
var p *Plugin
// init registers this plugin.
func init() { plugin.Register(pluginName, setup) }

// setup is the function that gets called when the config parser see the token "example". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "example".
func setup(c *caddy.Controller) error {
	c.Next() // Ignore "example" and give us the next token.
	if c.NextArg() {
		// If there was another token, return an error, because we don't have any configuration.
		// Any errors returned from this setup function should be wrapped with plugin.Error, so we
		// can present a slightly nicer error message to the user.
		return plugin.Error(pluginName, c.ArgErr())
	}

	c.OnStartup(func() error {
		//1. start docker client and read container info
		//2. launch goroutine for docker event handling
		p = NewPlugin()
		p.init()
		go p.handleEvent()
		return nil
	})

	c.OnShutdown(func() error {
		p.close()
		return nil
	})

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return &Docker{Next: next, Plugin:p}
	})

	// All OK, return a nil error.
	return nil
}

type Docker struct {
	Next plugin.Handler
	Plugin *Plugin
}

// ServeDNS implements the plugin.Handler interface. This method gets called when example is used
// in a Server.
func (e *Docker) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// This function could be simpler. I.e. just fmt.Println("example") here, but we want to show
	// a slightly more complex example as to make this more interesting.
	// Here we wrap the dns.ResponseWriter in a new ResponseWriter and call the next plugin, when the
	// answer comes back, it will print "example".
	log.Info("Received:", r.String())

	state := request.Request{W: w, Req: r}
	//if state.QClass() != dns.ClassCHAOS || state.QType() != dns.TypeTXT {
	//	return plugin.NextOrFailure(c.Name(), c.Next, ctx, w, r)
	//}
	name := state.Name()
	var ip net.IP
	if name != "my.io" {
		ip = e.Plugin.getIP(name)
	} else {
		ip = net.IPv4(127,0,0,1)
	}

	if ip == nil {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true
	hdr := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 10}
	record := dns.A{Hdr: hdr, A: ip}
	resp.Answer = append(resp.Answer, &record)
	err := w.WriteMsg(resp)
	if err != nil {
		log.Error(err)
	}
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *Docker) Name() string { return pluginName }

type Plugin struct {
	Docker *client.Client
	Ctx context.Context
	Map map[string]net.IP
}

func NewPlugin() *Plugin {
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	return &Plugin{
		Docker:cli,
		Ctx:ctx,
		Map: make(map[string]net.IP),
	}
}

//get all container's info
func (p *Plugin) init() {
	containers, err := p.Docker.ContainerList(p.Ctx, types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		p.cacheStart(container.ID)
		//name, ip := p.getHostnameAndIP(container.ID)
		//if len(name) != 0 && ip != nil {
		//	p.Map[name] = ip
		//}
	}
}

func (p *Plugin) getHostnameAndIP(containerId string) (string, net.IP) {
	info, _ := p.Docker.ContainerInspect(p.Ctx, containerId)
	return info.Config.Hostname, net.ParseIP(info.NetworkSettings.IPAddress)
}

func (p *Plugin) cacheStart(containerId string) {
	name, ip := p.getHostnameAndIP(containerId)
	if len(name) != 0 && ip != nil {
		p.Map[name] = ip
	}
}

func (p *Plugin) cacheStop(containerId string) {
	name, _ := p.getHostnameAndIP(containerId)
	delete(p.Map, name)
}

func (p *Plugin) getIP(name string) net.IP {
	return p.Map[name]
}

func (p *Plugin) handleEvent() {
	args := filters.NewArgs()
	args.Add("event", "start")
	args.Add("event", "stop")
	events, _ := p.Docker.Events(p.Ctx, types.EventsOptions {})
	for x := range events{
		str, _ := json.Marshal(x)
		fmt.Println(string(str))
		switch x.Action {
		case "start":
			p.cacheStart(x.Actor.ID)
		case "stop":
			p.cacheStop(x.Actor.ID)
		}
	}
}

func (p *Plugin) close() {
	_ = p.Docker.Close()
}