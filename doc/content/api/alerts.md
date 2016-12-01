---
date: 2016-09-29T11:02:01+02:00
title: Alerts
---

Skydive allows you to create alerts, based on queries on both topology graph
and flows.

## Alert evaluation
An alert can be specified through a [Gremlin](/api/gremlin) query or a
JavaScript expression. The alert will be triggered if it returns:

* true
* a non empty string
* a number different from zero
* a non empty array
* a non empty map

Gremlin example:

```console
$ skydive client alert create --expression "G.V().Has('Name', 'eth0', 'State', 'DOWN')"
{
  "UUID": "185c49ba-341d-41a0-6f96-f3224140b2fa",
  "Expression": "G.V().Has('Name', 'eth0', 'State', 'DOWN')",
  "CreateTime": "2016-12-29T13:29:05.273620179+01:00"
}
```

JavaScript example:

```console
$ skydive client alert create --expression "Gremlin(\"G.Flows().Has('Network.A', '192.168.0.1').Metrics().Sum()\").ABBytes > 1*1024*1024" --trigger "duration:10s"
{
  "UUID": "331b5590-c45d-4723-55f5-0087eef899eb",
  "Expression": "Gremlin(\"G.Flows().Has('Network.A', '192.168.0.1').Metrics().Sum()\").ABBytes > 1*1024*1024",
  "Trigger": "duration:10s",
  "CreateTime": "2016-12-29T13:29:05.197612381+01:00"
}
```

## Fields
* `Name`, the alert name (optional)
* `Description`, a description for the alert (optional)
* `Expression`, a Gremlin query or JavaScript expression
* `Action`, URL to trigger. Can be a [local file](/api/alerts#webhook) or a [WebHook](/api/alerts#script)
* `Trigger`, event that triggers the alert evaluation. Periodic alerts can be
   specified with `duration:5s`, for an alert that will be evaluated every 5 seconds.

## Notifications

When an alert is triggered, all the WebSocket clients will be notified with a
message of type `Alert` with a JSON object with the attributes:

* `UUID`, ID of the triggered alert
* `Timestamp`, timestamp of trigger
* `ReasonData`, the result of the alert evaluation. If `expression` is a
  Gremlin query, it will be the result of the query. If `expression` is a
  JavaScript statement, it will be the result of the evaluation of this
  statement.

In addition to the WebSocket message, an alert can trigger different kind of
actions.

### Webhook
A POST request is issued with the JSON message as payload.

### Script
A local file (prefixed by file://) to execute a script. It receives the JSON
message through stdin
