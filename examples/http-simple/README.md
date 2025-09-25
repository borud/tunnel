# Simple HTTP example

This example shows how you can create a http.Client that has a transport that uses the SSH tunnel.

This example uses the SSH Agent for authentication (specified by option `tunnel.WithAgent()`)

For simplicity we perform no host key checking (`tunnel.WithHostKeyCallback(ssh.InsecureIgnoreHostKey())`). To perform host key checking you have multiple options:

- Use per-hop HostKeyCallback if set.
- Else use per-hop KnownHostsPath if set.
- Else use global HostKeyCB from config if set.
- Else use global KnownHostsPath if set.
- Else fall back to defaultKnownHostsPath() → ~/.ssh/known_hosts
