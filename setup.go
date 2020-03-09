package docker

import (
	"context"
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/jwangsadinata/go-multimap/setmultimap"
	"github.com/miekg/dns"
	"net"
	"strings"
)

var pluginName = "docker"

var log = clog.NewWithPlugin(pluginName)
var p *Plugin
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
	if p == nil {
		p = NewPlugin()
		p.init()
		go p.handleEvent()
	}

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
	state := request.Request{W: w, Req: r}
	name := strings.Trim(state.QName(), ".")
	ips, exist := e.Plugin.Map.Get(name)
	log.Debugf("docker resolved: %s %v", name, ips)
	if !exist || ips == nil || len(ips) == 0 {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true
	hdr := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 10}
	for _, ip := range ips {
		record := dns.A{Hdr: hdr, A: net.ParseIP(ip.(string))}
		resp.Answer = append(resp.Answer, &record)
	}
	err := w.WriteMsg(resp)
	if err != nil {
		log.Error(err)
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *Docker) Name() string { return pluginName }

type Plugin struct {
	Docker *client.Client
	Ctx context.Context
	Map *setmultimap.MultiMap
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
		Map: setmultimap.New(),
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
	}
}

//find hostname from Label: hostname
func (p *Plugin) getHostnameAndIP(containerId string) (string, net.IP) {
	info, _ := p.Docker.ContainerInspect(p.Ctx, containerId)
	ip := net.ParseIP(info.NetworkSettings.IPAddress)
	hostname := info.Config.Labels["hostname"]
	return hostname, ip
}

func (p *Plugin) cacheStart(containerId string) {
	name, ip := p.getHostnameAndIP(containerId)
	if len(name) != 0 && ip != nil {
		p.Map.Put(name, ip.String())
		log.Info("cache:", name, ip)
	}
}

func (p *Plugin) cacheStop(containerId string) {
	name, ip := p.getHostnameAndIP(containerId)
	if len(name) != 0 {
		p.Map.Remove(name, ip.String())
		log.Info("remove cache:", name)
	}
}

//func (p *Plugin) getIPs(name string) map[string]bool {
//	return p.Map.Get(name)
//}

func (p *Plugin) handleEvent() {
	args := filters.NewArgs()
	args.Add("event", "start")
	args.Add("event", "stop")
	events, _ := p.Docker.Events(p.Ctx, types.EventsOptions {Filters: args})
	for x := range events{
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
