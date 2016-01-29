# Gremlin Server Client for Go

This library will allow you to connect to any graph database that supports TinkerPop3 using `Go`. This includes databases like Titan and Neo4J. TinkerPop3 uses Gremlin Server to communicate with clients using either WebSockets or REST API. This library talks to Gremlin Server using WebSockets.


Installation
==========
```
go get github.com/go-gremlin/gremlin
```

Usage
======
Export the list of databases you want to connect to as `GREMLIN_SERVERS` like so:-
```bash
export GREMLIN_SERVERS="ws://server1:8182, ws://server2:8182"
```

Import the library eg `import "github.com/go-gremlin/gremlin"`.

Parse and save your cluster of services. You only need to do this once before submitting any queries (Perhaps in `main()`):-
```go
	if err := gremlin.NewCluster(); err != nil {
		// handle error here
	}
```

Instead of using an environment variable, you can also pass the servers directly to `NewCluster()`. This is more convenient in development. For example:-
```go
	if err := gremlin.NewCluster("ws://dev.local:8182", "ws://staging.local:8182"); err != nil {
		// handle error
	}
```

To actually run queries against the database, make sure the package is imported and issue a gremlin query like this:-
```go
	data, err := gremlin.Query(`g.V()`).Exec()
	if err != nil  {
		// handle error
	}
```
`data` is a `JSON` array in bytes `[]byte` if any data is returned otherwise it is `nil`. For example you can print it using:-
```go
	fmt.Println(string(data))
```
or unmarshal it as desired.

You can also execute a query with bindings like this:-
```go
	data, err := gremlin.Query(`g.V().has("name", userName).valueMap()`).Bindings(gremlin.Bind{"userName": "john"}).Exec()
```
