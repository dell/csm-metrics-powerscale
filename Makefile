.PHONY: all
all: help

help:
	@echo
	@echo "The following targets are commonly used:"
	@echo
	@echo "build    - Builds the code locally"
	@echo "clean    - Cleans the local build"
	@echo "docker   - Builds Docker images"
	@echo "push     - Pushes Docker images to a registry"
	@echo "check    - Runs code checking tools: lint, format, gosec, and vet"
	@echo "test     - Runs the unit tests"
	@echo

.PHONY: build
build: 
	@$(foreach svc,$(shell ls cmd), CGO_ENABLED=0 GOOS=linux go build -o ./cmd/${svc}/bin/service ./cmd/${svc}/;)

.PHONY: clean
clean:
	rm -rf cmd/*/bin

.PHONY: generate
generate:
	go generate ./...

.PHONY: test
test:
	go test -count=1 -cover -race -timeout 30s -short `go list ./... | grep -v mocks`

.PHONY: docker
docker: download-csm-common
	$(eval include csm-common.mk)
	SERVICE=cmd/metrics-powerscale docker build -t csm-metrics-powerscale -f Dockerfile --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) cmd/metrics-powerscale/

.PHONY: push
push:
	docker push ${DOCKER_REPO}/csm-metrics-powerscale\:latest

.PHONY: tag
tag:
	docker tag csm-metrics-powerscale\:latest ${DOCKER_REPO}/csm-metrics-powerscale\:latest

.PHONY: check
check:
	./scripts/check.sh ./cmd/... ./opentelemetry/... ./internal/...

.PHONY: download-csm-common
download-csm-common:
	curl -O -L https://raw.githubusercontent.com/dell/csm/main/config/csm-common.mk
