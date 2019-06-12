# Skydive Kubernetes

## Usage

To install run:

```
kubectl apply -f skydive.yaml
```

To expose UI management port:

```
kubectl port-forward service/skydive-analyzer 8082:8082
```

To uninstall run:

```
kubectl delete -f skydive.yaml
```

## Configuration

Change analyzer listening port from 8082 to 18082:

```
cat skydive.yaml | sed -e s/8082/18082/ | kubectl apply -f -
```

Change agent listening port from 8081 to 18081:

```
cat skydive.yaml | sed -e s/8081/18081/ | kubectl apply -f -
```

Change elasticsearch port from 9200 to 19200:

```
cat skydive.yaml | sed -e s/9200/19200/ | kubectl apply -f -
```

Change etcd port from 12379/12380 to 22379/22380:

```
cat skydive.yaml | sed -e s/12379/22379/ -e s/12380/22380/ | kubectl apply -f -
```
