---
date: 2016-05-06T11:02:01+02:00
title: How to deploy Skydive with Docker
---

## Docker

A Docker image is available on the [Skydive Docker Hub account](https://hub.docker.com/r/skydive/).

To start the analyzer :
```console
docker run -p 8082:8082 -p 2379:2379 skydive/skydive analyzer
```

To start the agent :
```console
docker run --privileged --pid=host --net=host -p 8081:8081 -v /var/run/docker.sock:/var/run/docker.sock skydive/skydive agent
```

## Docker Compose

[Docker Compose](https://docs.docker.com/compose/) can also be used to automatically start
an Elasticsearch container, a Skydive analyzer container and a Skydive agent container. The service
definition is located in the `contrib/docker` folder of the Skydive sources.

```console
docker-compose up
```
