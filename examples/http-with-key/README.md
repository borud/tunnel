# HTTP tunneling with key file

This example shows how you can create a http.Client that has a transport that uses the SSH tunnel. In this example we show how you can load a key from a file.  We assume that you have not encrypted the SSH key file with a password.

## Running this code

### Generating a key for testing

In order to test this you will want to create an SSH key:

```shell
ssh-keygen -t ed25519 -f ./id_ed25519 -N ""
```

This will produce two files in the current directory.  

- `id_ed25519` contains the private key, and
- `id_ed25519.pub` contains the public key.

The `-N` option specifies that there is no password.

### Add public key to `authorized_keys`

You can then add the public key in `id_ed25519.pub` to your `authorized_keys` file on the target host. *Don't forget to remove the key from `authorized_keys` after you perform the test unless you want that key to be there*.

### Run it

In the example below, replace `example.com` with the host you want to tunnel through.

```shell
go run main.go -key id_ed25519 example.com https://news.ycombinator.com/
```

## Using `WithKey` instead of `WithKeyFile`

In some circumstances you might want to provide an SSH key (in PEM format) directly rather than reading it from a file.  In that case you can use the `WithKey` option instead of `WithKeyFile`.

## Generating key in Go

If you want to generate the key in Go rather than using `ssh-keygen` you can do this like so:

```go
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/pem"
    "fmt"
    "os"
    "golang.org/x/crypto/ssh"
)

func main() {
    // Generate ed25519 keypair
    _, priv, _ := ed25519.GenerateKey(rand.Reader)

    // Marshal private key into PEM
    privBytes, _ := ssh.MarshalPrivateKey(priv, "")
    pemBlock := &pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: privBytes}
    pem.Encode(os.Stdout, pemBlock)

    // Marshal public key
    pub, _ := ssh.NewPublicKey(priv.Public().(ed25519.PublicKey))
    fmt.Printf("%s\n", ssh.MarshalAuthorizedKey(pub))
}
```
