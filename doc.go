// Package tunnel implements a tunnel to another machine from which we can
// Dial other machines or Listen to remote ports. For now this library won't
// check host keys (it just accepts all) and for simplicity it assumes that
// you are using an ssh-agent to access your ssh keys.
//
// Typical use:
//
// First we create the tunnel
//
//	tunnel, err := tunnel.Create(tunnel.Config{
//		Hops: []string{
//			"bob@bastion.example.com:22",
//			"alice@inside.example.com:22",
//		},
//	})
//	...
//
// Then we connect using the tunnel
//
//	conn, err := tunnel.Dial("tcp", "service.example.com:4711")
//	...
//
// If everything went according to plan you now have a tunnel that terminates
// at inside.example.com (since it is the last hop) and connects from there to
// port 4711 on service.example.com
//
// You can also listen on the remote endpoint.
//
//	listener, err := tunnel.Listen("tcp", ":80")
//	...
package tunnel
