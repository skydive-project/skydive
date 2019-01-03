# OpenShift template for skydive agent

This OpenShift template allows you to instantiate skydive in OpenShift.

Assuming you work on project skydive:

```
oc new-project skydive
```

Install the template

```
# adjust VERSION for the current version - for example: v0.20.1 or master
VERSION=v0.20.1
oc create -f https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-template.yaml
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

# FAQ

## How to deploy the skydive agent on all nodes

The current template only deploys the skydive agents to default compute nodes with selectors in `osm_default_node_selector` or in  `_openshift_node_group_name_` as there is no **node-selector** defined in the `skydive-template.yaml`.
You can execute the command below to add the skydive agent to the other nodes.

`oc patch namespace skydive -p '{"metadata": {"annotations": {"openshift.io/node-selector": ""}}}'`
