#!/bin/bash

# Deployment (network) version - for example: v0.20.1 or master
VERSION=${SKYDIVE_VARIABLE:-master}
USE_LOCAL_YAML=${SKYDIVE_USE_LOCAL_YAML:-false}

while getopts ":lh" opt; do
  echo $ops
  case ${opt} in
    l ) USE_LOCAL_YAML=true
      ;;
    \?)
      echo "Usage:"
      echo "    -h    Display this help message"
      echo "    -l    Deploy using local skydive-template.yaml file"
      exit 0
      ;;      
  esac
done
shift $((OPTIND -1))

# delete skydive project if it exists (to get new frech deployment)
delete_skydive_project_if_exists() {
PROJECT_SKYDIVE=$(oc get project | grep skydive)
if [ ! -z "$PROJECT_SKYDIVE" ]; then 
  oc delete project skydive
  while : ; do
    PROJECT_SKYDIVE=$(oc get project | grep skydive)
    if [ -z "$PROJECT_SKYDIVE" ]; then break; fi 
    sleep 1
  done
fi
}

# create and switch context to skydive project
create_skydive_project() {
  oc adm new-project --node-selector='' skydive
  oc project skydive
}

# set credentials for skydive deployment 
set_credentials() {
  # analyzer and agent run as privileged container
  oc adm policy add-scc-to-user privileged -z default
  # analyzer need cluster-reader access get all informations from the cluster
  oc adm policy add-cluster-role-to-user cluster-reader -z default
}

# deoploy skydive
deploy_skydive() {
  if [ $USE_LOCAL_YAML = "true" ] ; then
    DEPLOY_YAML=skydive-template.yaml
  else
    DEPLOY_YAML=https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-template.yaml
  fi

  echo -e "\nDeploying $DEPLOY_YAML\n"
  oc process -f $DEPLOY_YAML | oc apply -f -
}

# deoploy skydive-flow-exporter
deploy_skydive-flow-exporter() {
  if [ $USE_LOCAL_YAML = "true" ] ; then
    DEPLOY_YAML=skydive-flow-exporter-template.yaml
  else
    DEPLOY_YAML=https://raw.githubusercontent.com/skydive-project/skydive/${VERSION}/contrib/openshift/skydive-flow-exporter-template.yaml
  fi

  echo -e "\nDeploying $DEPLOY_YAML\n"
  oc process -f $DEPLOY_YAML | oc apply -f -
}

# print pod status
print_pods_status () {
  echo -e "\n"
  oc get pods
}

# print usage insturctions
print_usage_insturctions () {
  SKYDIVE_ANALYZER_ROUTE=$(oc get route skydive-analyzer -o jsonpath='http://{.spec.host}')
  SKYDIVE_UI_ROUTE=$(oc get route skydive-ui -o jsonpath='http://{.spec.host}')
  
  echo -e "\n\nTo access skydive UI point browser to: $SKYDIVE_UI_ROUTE\n\nNote (1): Wait for pods to become ready and agants to communicate with analyzer\nNote (2): You need to logout and set endpoint to: $SKYDIVE_ANALYZER_ROUTE\n      use user=admin and password=password"
}

# main
main() {
  delete_skydive_project_if_exists
  create_skydive_project
  set_credentials
  deploy_skydive
  deploy_skydive-flow-exporter
  print_pods_status
  print_usage_insturctions
}

main
