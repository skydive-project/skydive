include ../../../.mk/k8s.mk

# must be within the range of k8s nodePort
KIBANA_NODEPORT?=30030
KIBANA_PORT?=5601
ES_SERVICE?=elasticsearch-master
ES_PORT?=9200

.PHONY: repo
repo: $(TOOLSBIN)/helm
	helm repo remove elastic https://helm.elastic.co 2>/dev/null || true
	helm repo add elastic https://helm.elastic.co
	helm repo update

.PHONY: uninstall
uninstall: $(TOOLSBIN)/helm
	helm uninstall kibana 2>/dev/null || true

.PHONY: install
install: $(TOOLSBIN)/helm
	helm install kibana elastic/kibana \
		--set elasticsearchHosts="http://${ES_SERVICE}:${ES_PORT}" \
		--set service.port=${KIBANA_PORT} \
		--set service.type=NodePort \
		--set service.nodePort=${KIBANA_NODEPORT}

.PHONY: status
status: $(TOOLSBIN)/kubectl
	kubectl get all -l app=kibana

.PHONY: logs
logs: $(TOOLSBIN)/kubectl
	kubectl logs -f -l app=kibana

.PHONY: verify
verify:
	curl http://localhost:${KIBANA_NODEPORT}/app/dashborads

.PHONY: port-forward
port-forward: $(TOOLSBIN)/kubectl
	kubectl port-forward service/kibana-kibana ${KIBANA_PORT}:${KIBANA_PORT}

.PHONY: help
help:
	@echo "Kibana is running at: http://localhost:${ELASTIC_NODEPORT}"
