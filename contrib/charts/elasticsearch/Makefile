include ../../../.mk/k8s.mk

# must be within the range of k8s nodePort
ES_NODEPORT?=30020
ES_PORT?=9200
ES_SERVICE?=elasticsearch-master

.PHONY: repo
repo: $(TOOLSBIN)/helm
	helm repo remove elastic https://helm.elastic.co 2>/dev/null || true
	helm repo add elastic https://helm.elastic.co
	helm repo update

.PHONY: uninstall
uninstall: $(TOOLSBIN)/helm
	helm uninstall elasticsearch 2>/dev/null || true

.PHONY: install
install: $(TOOLSBIN)/helm
	helm install elasticsearch elastic/elasticsearch \
		--set masterService=${ES_SERVICE} \
		--set service.port=${ES_PORT} \
		--set service.type=NodePort \
		--set service.nodePort=${ES_NODEPORT}

.PHONY: status
status: $(TOOLSBIN)/kubectl
	kubectl get all -l app=elasticsearch-master

.PHONY: logs
logs: $(TOOLSBIN)/kubectl
	kubectl logs -f -l app=elasticsearch-master

.PHONY: verify
verify:
	curl http://localhost:${ES_NODEPORT}

.PHONY: port-forward
port-forward: $(TOOLSBIN)/kubectl
	kubectl port-forward service/${ES_SERVICE} ${ES_PORT}:${ES_PORT}

.PHONY: delete
delete:
	curl -X DELETE http://localhost:${ES_NODEPORT}/_all

.PHONY: indices
indices:
	curl http://localhost:${ES_NODEPORT}/_cat/indices

.PHONY: help
help:
	@echo "ElasticSearch is running at: http://localhost:${ES_NODEPORT}"
