rpc2
====

[![GoDoc](https://godoc.org/github.com/cenkalti/rpc2?status.png)](https://godoc.org/github.com/cenkalti/rpc2)
[![Build Status](https://travis-ci.org/cenkalti/rpc2.png)](https://travis-ci.org/cenkalti/rpc2)

rpc2 is a fork of net/rpc package in the standard library.
The main goal is to add bi-directional support to calls.
That means server can call the methods of client.
This is not possible with net/rpc package.
In order to do this it adds a `*Client` argument to method signatures.

Install
--------

    go get github.com/cenkalti/rpc2

Example server
---------------

```go
type Args struct{ A, B int }
type Reply int

srv := rpc2.NewServer()
srv.Handle("add", func(client *rpc2.Client, args *Args, reply *Reply) error {
        // Reversed call (server to client)
        var rep Reply
        client.Call("mult", Args{2, 3}, &rep)
        fmt.Println("mult result:", rep)

        *reply = Reply(args.A + args.B)
        return nil
})

lis, _ := net.Listen("tcp", "127.0.0.1:5000")
srv.Accept(lis)
```

Example Client
---------------

```go
type Args struct{ A, B int }
type Reply int

conn, _ := net.Dial("tcp", "127.0.0.1:5000")

clt := rpc2.NewClient(conn)
clt.Handle("mult", func(client *rpc2.Client, args *Args, reply *Reply) error {
        *reply = Reply(args.A * args.B)
        return nil
})
go clt.Run()

var rep Reply
clt.Call("add", Args{1, 2}, &rep)
fmt.Println("add result:", rep)
```
