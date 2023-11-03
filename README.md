# tunnel

`tunnel` is a very simple library that allows you to create multi-hop SSH tunnels. From the endpoint
of the tunnel you can then `Dial()` to create network connections, or you can `Listen()` for
incoming connections.

You are responsible for closing any connections or listeners you make. The tunnel doesn't keep track
of any connections you might have opened.

You can create multiple connections through the same tunnel.

## A note on Listen ports

When you want to `Listen` to remote ports that should be externally available, you have to make sure
that the SSH daemon is configured to allow this.  Please review the `GatewayPorts` configuration
option in `sshd_config`.  If you were too lazy to read this paragraph and are just looking for a cut
and paste, the config is:

```text
GatewayPorts yes
```
