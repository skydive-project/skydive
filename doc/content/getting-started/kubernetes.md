---
date: 2016-05-14T11:02:01+02:00
title: How to deploy Skydive with kubernetes
---

## Kubernetes deployment

Skydive provides a Kubernetes
[file](https://github.com/skydive-project/skydive/blob/master/contrib/kubernetes/skydive.yaml)
which can be used to deploy Skydive. It will deploy an Elasticsearch,
a Skydive analyzer and Skydive Agent on each Kubernetes nodes. Once you will
have Skydive deployment on top on your Kubernetes cluster you will be able to
monitor, capture, troubleshoot your container networking stack.

A skydive Analyzer [Kubernetes service](http://kubernetes.io/docs/user-guide/services/)
is created and exposes ports for Elasticsearch and the Analyzer:

* Elasticsearch: 9200
* Analyzer: 8082

[Kubernetes DaemonSet](http://kubernetes.io/docs/admin/daemons/) is used for
Agents in order to have one Agent per node.

## Creation

```console
kubectl create -f skydive.yaml
```

Once you have your environment set up, going to the Analyzer service
should show similar to the following capture.

![WebUI Capture](/images/kubernetes-two-nodes.png)
