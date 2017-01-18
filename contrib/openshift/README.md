# OpenShift template for skydive agent

This OpenShift template allows you to instantiate skydive in OpenShift.

Assuming you work on project skydive:

```
oc new-project skydive
```

Install the template

```
oc create -f skydive-template.yaml
```

You need the DeploymentConfig to run in privileged:

```
oc adm policy add-scc-to-user privileged system:serviceaccount:skydive:default
```

Instanciate the template:

```
oc new-app --template=skydive
```

Check that everything is working and created:

```
oc get pods
oc get ds
```

Expose your route:

```
oc delete route skydive-analyzer
oc expose svc skydive-analyzer
```

