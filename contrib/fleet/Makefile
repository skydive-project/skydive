include ../../.mk/k8s.mk

.PHONY: install uinstall status logs
install uninstall status logs: $(TOOLSBIN)/k3d $(TOOLSBIN)/helm $(TOOLSBIN)/kubectl $(TOOLSBIN)/jq
	./skydive.sh $@

.PHONY: broker east west
broker east west: $(TOOLSBIN)/kubectl
	kubectl --context k3d-$@ port-forward service/skydive-analyzer 8082:8082
