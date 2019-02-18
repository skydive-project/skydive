# Skydive Kubernetes

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
