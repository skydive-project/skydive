include ../../../.mk/k8s.mk

# must be within the range of k8s nodePort
ANALYZER_NODEPORT?=30082
ANALYZER_PORT?=8082
ANALYZER_SERVICE?=skydive-analyzer
ETCD_NODEPORT?=30079
NEWUI_NODEPORT?=30080

.PHONY: uninstall
uninstall: $(TOOLSBIN)/helm
	helm uninstall skydive-analyzer 2>/dev/null || true

.PHONY: install
install: $(TOOLSBIN)/helm
	helm install skydive-analyzer . \
		--set service.type=NodePort \
		--set service.port=${ANALYZER_PORT} \
		--set service.nodePort=${ANALYZER_NODEPORT} \
		--set etcd.nodePort=${ETCD_NODEPORT} \
		--set elasticsearch.enabled=true \
		--set newui.enabled=true \
		--set newui.nodePort=${NEWUI_NODEPORT} \

.PHONY: status
status: $(TOOLSBIN)/kubectl
	kubectl get all -l app=skydive-analyzer

.PHONY: logs
logs: $(TOOLSBIN)/kubectl
	kubectl logs -f -l app=skydive-analyzer -c skydive-analyzer

.PHONY: verify
verify:
	curl http://localhost:${ANALYZER_NODEPORT}

.PHONY: port-forward
port-forward: $(TOOLSBIN)/kubectl
	kubectl port-forward service/${ANALYZER_SERVICE} ${ANALYZER_PORT}:${ANALYZER_PORT}

.PHONY: help
help:
	@echo "Skydive Analyzer is running at: http://localhost:${ANALYZER_NODEPORT}"
	@echo "Skydive ETCD is running at: http://localhost:${ETCD_NODEPORT}"
	@echo "Skydive NewUI is running at: http://localhost:${NEWUI_NODEPORT}"
