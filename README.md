# tunnel

[![Go Reference](https://pkg.go.dev/badge/github.com/borud/tunnel.svg)](https://pkg.go.dev/github.com/borud/tunnel)

`tunnel` is a very simple library that allows you to create multi-hop SSH
tunnels. From the endpointof the tunnel you can then `Dial()` to create network
connections, or you can `Listen()` for incoming connections.

This library supports both using the SSH Agent to load any keys you might need as well as loading keys from files or from `[]byte` slices in PEM format.

Per default the implementation will keep track of any connections or listeners you make.  If you shut this off you have to manage the connections yourself. I recommend using the default behavior (library tracks connections).

You can create multiple connections through the same tunnel.

## Usage Examples

Please have a look in the [examples](examples) directory for some usage examples.

## Typical use

### Import

Add the following import and run `go mod tidy` to add tunnel to your project.

```go
import "github.com/borud/tunnel"
```

### Creating the tunnel

This example just creates a tunnel with two hops

```go
tun, err := tunnel.Create(
  tunnel.WithHop("user@first.example.com"), 
  tunnel.WithHop("user@second.example.com"), 
  tunnel.WithAgent(), 
  tunnel.WithHostKeyCallback(ssh.InsecureIgnoreHostKey()),
)
```

### Dial

You can `Dial` to create a new connection over the tunnel like so:

```go
  conn, err := tun.Dial("tcp", "service.example.com:4711")
```

If everything went according to plan you now have a tunnel that terminates at
second.example.com (since it is the last hop) and connects from there to port
4711 on service.example.com

### Listen

You can also listen on the remote endpoint.

```go
listener, err := tunnel.Listen("tcp", ":80")
```

## A note on Listen ports

When you want to `Listen` to remote ports that should be externally available,
you have to make sure that the SSH daemon is configured to allow this.  Please
review the `GatewayPorts` and `AllowTcpForwarding` configuration options in
`sshd_config`.  If you were too lazy to read this paragraph and are just
looking for a cut and paste, the config is:

```text
GatewayPorts yes
AllowTcpForwarding yes
```
