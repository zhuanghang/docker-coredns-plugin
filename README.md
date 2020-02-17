# docker-coredns-plugin
docker coredns plugin: resolve docker contains with label "hostname=xxx" to its ip address.
this is for local docker env service discovery

# how to build coredns binary with this plugin

1. `git clone https://github.com/coredns/coredns`
2. cd coredns
3. add `docker:github.com/feiyanke/docker-coredns-plugin` into plugin.cfg (may after 'acl' line)
4. `make`
5. add plugin `docker` into Corefile, for example
```
.:53 {
    docker
    forward . 8.8.8.8:53
    log
}
```
6. run coredns
