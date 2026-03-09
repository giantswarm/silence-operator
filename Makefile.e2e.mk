##@ E2E Tests

E2E_CLUSTER_NAME ?= silence-operator-e2e
E2E_IMAGE_NAME ?= silence-operator
E2E_IMAGE_TAG ?= e2e-test
E2E_HELM_RELEASE ?= silence-operator
E2E_HELM_NAMESPACE ?= silence-operator
E2E_AM_NAMESPACE ?= alertmanager

.PHONY: e2e-create-cluster
e2e-create-cluster: ## Create a kind cluster for e2e tests
	kind create cluster --name $(E2E_CLUSTER_NAME) --wait 60s

.PHONY: e2e-build-image
e2e-build-image: ## Build the operator image and load it into kind
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o silence-operator ./cmd/
	docker build -t $(E2E_IMAGE_NAME):$(E2E_IMAGE_TAG) .
	kind load docker-image $(E2E_IMAGE_NAME):$(E2E_IMAGE_TAG) --name $(E2E_CLUSTER_NAME)

.PHONY: e2e-deploy-alertmanager
e2e-deploy-alertmanager: ## Deploy Alertmanager into the kind cluster
	kubectl apply -f e2e/manifests/alertmanager.yaml
	kubectl -n $(E2E_AM_NAMESPACE) rollout status deployment/alertmanager --timeout=120s

.PHONY: e2e-deploy-operator
e2e-deploy-operator: ## Deploy the silence-operator via Helm
	kubectl create namespace $(E2E_HELM_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	helm install $(E2E_HELM_RELEASE) helm/silence-operator \
		--namespace $(E2E_HELM_NAMESPACE) \
		--set image.registry="" \
		--set image.name=$(E2E_IMAGE_NAME) \
		--set image.tag=$(E2E_IMAGE_TAG) \
		--set alertmanagerAddress=http://alertmanager.$(E2E_AM_NAMESPACE).svc.cluster.local:9093 \
		--set networkPolicy.enabled=false \
		--set podMonitor.enabled=false \
		--wait --timeout 120s

.PHONY: e2e-setup
e2e-setup: e2e-create-cluster e2e-build-image e2e-deploy-alertmanager e2e-deploy-operator ## Set up the full e2e environment

.PHONY: e2e-run
e2e-run: ## Run the e2e tests
	go test -v -tags=e2e -count=1 -timeout 15m ./e2e/...

.PHONY: e2e-test
e2e-test: e2e-setup e2e-run ## Run full e2e setup + tests

.PHONY: e2e-clean
e2e-clean: ## Delete the kind cluster
	kind delete cluster --name $(E2E_CLUSTER_NAME) || true
