import { Alert, API, Client, Capture, NE } from "./api"

var client: Client;
if (this.client === undefined) {
    client = new Client({
        "baseURL": "http://localhost:8082",
        "username": "admin",
        "password": "password"
    });
} else {
  client = this.client
}

client.login().then(function () {
  return client.captures.list()
}, function (error) {
  console.log("Failed to log in: " + error);
})
.then(function (captures) {
  console.log("Listing captures:")
  console.log(captures);
}, function (error) {
  console.log("Error while listing captures: " + error);
})
.then(function () {
  return client.alerts.list()
})
.then(function (alerts) {
  console.log("Listing alerts:")
  console.log(alerts);
}, function (error) {
  console.log("Error while listing alerts: " + error);
})
.then(function () {
  var capture2 = new Capture();
  capture2.GremlinQuery = "g.V().Has('Name', 'noname')";
  return client.captures.create(capture2);
})
.then(function (capture: Capture) {
  console.log("Creating capture:")
  console.log(capture);
  return client.captures.delete(capture.UUID);
}, function (error) {
  console.log("Error while creating capture: " + error);
})
.then(function () {
  return client.G.V().Has("Type", "host")
})
.then(function (nodes) {
  console.log("Nodes: " + nodes);
  console.log(nodes[0].ID)
})
.then(function () {
  return client.G.E().Has("Host", NE(1234))
})
.then(function (edges) {
  console.log("Edges: " + edges);
})
.then(function () {
  return client.G.Flows().Has('Application', 'TCP')
})
.then(function (flows) {
  console.log("Flows: " + flows);
})
