TOOLSBIN:=/usr/local/bin
export PATH:=$(TOOLSBIN):${PATH}

$(TOOLSBIN)/k3d:
	curl -s https://raw.githubusercontent.com/rancher/k3d/main/install.sh | TAG=v3.0.0 bash

$(TOOLSBIN)/kind:
	GOBIN=$(TOOLSBIN) GO111MODULE=on sudo -E $(shell which go) get sigs.k8s.io/kind@v0.8.1

$(TOOLSBIN)/helm:
	curl -fsSL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | HELM_INSTALL_DIR=$(TOOLSBIN) sudo -E bash -

$(TOOLSBIN)/kubectl:
	sudo -E curl -fsSL -o $(TOOLSBIN)/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.20.1/bin/linux/amd64/kubectl && \
		sudo -E chmod a+x $(TOOLSBIN)/kubectl

$(TOOLSBIN)/jq:
	sudo wget -O $@ https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 && \
		sudo chmod a+x $@

# NodePorts are in the 30000-32767 range by default, which means a NodePort is
# unlikely to match a service’s intended port (for example, 8080 may be exposed
# as 31020).
K3D_NODEPORTS?=30000-30100

.PHONY: k3d
k3d: k3d-delete k3d-create

.PHONY: k3d-create
k3d-create: $(TOOLSBIN)/k3d
	k3d cluster create manager -p "${K3D_NODEPORTS}:${K3D_NODEPORTS}@server[0]" --agents 2

.PHONY: k3d-delete
k3d-delete: $(TOOLSBIN)/k3d
	k3d cluster delete manager 2>/dev/null || true

.PHONY: kind
kind: kind-delete kind-create

.PHONY: kind-create
kind-create: $(TOOLSBIN)/kind
	kind create cluster --name kind

.PHONY: kind-delete
kind-delete: $(TOOLSBIN)/kind
	kind delete cluster --name kind 2>/dev/null || true
