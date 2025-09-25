# Multihop tunnel

This example shows how you can create a http.Client that has a transport that uses the SSH tunnel.  You can specify multiple hops in a comma separated list.

```shell
go run main.go host1,host2,host3 https://news.ycombinator.com/
```
