# tunnel

[![Go Reference](https://pkg.go.dev/badge/github.com/borud/tunnel.svg)](https://pkg.go.dev/github.com/borud/tunnel)

*Note that the state of this library is experimental*.

`tunnel` is a very simple library that allows you to create multi-hop SSH tunnels. From the endpoint
of the tunnel you can then `Dial()` to create network connections, or you can `Listen()` for
incoming connections.

This library uses the `ssh-agent` to load any keys you might need. If you need to load keys from
files, let me know and I'll probably add support for it.

You are responsible for closing any connections or listeners you make. The tunnel doesn't keep track
of any connections you might have opened.

You can create multiple connections through the same tunnel.

## Typical use

### Import

Add the folliwing import and run `go mod tidy` to add tunnel to your project.

```go
import "github.com/borud/tunnel"
```

### Creating the tunnel

```go
tunnel, err := tunnel.Create(tunnel.Config{
    Hops: []string{
        "bob@bastion.example.com:22",
        "alice@inside.example.com:22",
    },
})
```

### Dial

You can `Dial` to create a new connection over the tunnel like so:

```go
  conn, err := tunnel.Dial("tcp", "service.example.com:4711")
```

If everything went according to plan you now have a tunnel that terminates at
inside.example.com (since it is the last hop) and connects from there to port
4711 on service.example.com

### Listen

You can also listen on the remote endpoint.

```go
listener, err := tunnel.Listen("tcp", ":80")
```

## A note on Listen ports

When you want to `Listen` to remote ports that should be externally available, you have to make sure
that the SSH daemon is configured to allow this.  Please review the `GatewayPorts` configuration
option in `sshd_config`.  If you were too lazy to read this paragraph and are just looking for a cut
and paste, the config is:

```text
GatewayPorts yes
```
