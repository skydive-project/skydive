#### For quick default deployment use `deploy_skydive_ocp.sh` script 

# OpenShift template for skydive agent

This OpenShift template allows you to instantiate skydive in OpenShift. 

**Important**: For Skydive installation you need cluster-admin privileges. During the installation you have to grant same privileges to skydive.

#### Create a new project with an empty node selector to have Skydive running on all nodes. 

```
oc adm new-project --node-selector='' skydive
oc project skydive
```

####  Skype analyzer and agent need  extended  privileges

```
# analyzer and agent run as privileged container
oc adm policy add-scc-to-user privileged -z default
# analyzer need cluster-reader access get all informations from the cluster
oc adm policy add-cluster-role-to-user cluster-reader -z default
```


####  Deploy skydive analyzer and agents from OpenShift template

```
# adjust VERSION for the current version - for example: v0.20.1 or master
VERSION=master
oc process -f https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-template.yaml | oc apply -f -
```

####  Deploy flow exporter from OpenShift template

```
# adjust VERSION for the current version - for example: v0.20.1 or master
VERSION=master
oc process -f https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-flow-exporter-template.yaml | oc apply -f -
```

#### Check that everything is working and created:

 - Overall status:  
   `oc status`
 - List all pods:  
   `oc get pods`
 - List all daemonsets:  
   `oc get daemonset`
 - List all services:  
   `oc get services`
 - List all routes:  
   `oc get routes`

#### UI Access 

Perform the Following steps to access skydive UI 
 - point your browser to skydive UI end-point  
 `oc get route skydive-ui -o jsonpath='http://{.spec.host}'`
 - logout using small person icon on screen top right  
 - Specify analyzer endpoint   
 `oc get route skydive-analyzer -o jsonpath='http://{.spec.host}'`
 - Use username = `admin` , Password = `password`  
 - press **sign-in**  

# Installation parameters

The skydive template provide some installation 

```
$ VERSION=v0.20.1
$ oc process --parameters -f https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-template.yaml
NAME                    DESCRIPTION                              GENERATOR           VALUE
SKYDIVE_LOGGING_LEVEL   Loglevel of Skydive agent and analyzer                       INFO

# Installation with loglevel debug:
$ oc process --param=SKYDIVE_LOGGING_LEVEL=DEBUG -f https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-template.yaml  | oc apply -f -
```
