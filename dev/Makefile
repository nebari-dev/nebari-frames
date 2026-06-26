# Local end-to-end testing on kind. Delegates the Nebari stack (Envoy Gateway,
# cert-manager, Keycloak) to the nebari-operator dev scripts. Run from this dir.
.PHONY: up up-standalone down image-load install install-standalone

CLUSTER ?= frames-dev
IMAGE ?= ghcr.io/nebari-dev/nebari-frames:dev
NAMESPACE ?= nebari-frames

up:
	@echo "Bring up a kind cluster with the full Nebari stack (operator dev scripts),"
	@echo "then run 'make image-load install'. See README for the operator-repo prerequisites."
	kind create cluster --name $(CLUSTER) || true

up-standalone:
	kind create cluster --name $(CLUSTER) || true
	$(MAKE) image-load install-standalone

image-load:
	cd .. && make image IMAGE_TAG=dev
	kind load docker-image $(IMAGE) --name $(CLUSTER)

install:
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl label namespace $(NAMESPACE) nebari.dev/managed=true --overwrite
	helm upgrade --install frames ../chart -n $(NAMESPACE) -f ../examples/nebari-values.yaml

install-standalone:
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install frames ../chart -n $(NAMESPACE) -f ../examples/standalone-values.yaml

down:
	kind delete cluster --name $(CLUSTER)
