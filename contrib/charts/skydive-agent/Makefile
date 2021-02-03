include ../../../.mk/k8s.mk

ANALYZER_SERVICE?=skydive-analyzer
ANALYZER_PORT?=8082

.PHONY: uninstall
uninstall: $(TOOLSBIN)/helm
	helm uninstall skydive-agent 2>/dev/null || true

.PHONY: install
install: $(TOOLSBIN)/helm
	helm install skydive-agent . \
		--set analyzer.host=${ANALYZER_SERVICE}:${ANALYZER_PORT}

.PHONY: status
status: $(TOOLSBIN)/kubectl
	kubectl get all -l app=skydive-agent

.PHONY: logs
logs: $(TOOLSBIN)/kubectl
	kubectl logs -f -l app=skydive-agent
