TAG := $(shell git describe --always --dirty)
REPO := addme

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run \
		--deadline=10m \
		--disable=unused,deadcode \
		--enable=gofmt,errcheck,govet,unconvert,gosimple,staticcheck,bodyclose,misspell,goimports

test:
	go test ./... -v

format:
	go fmt ./...

build:
	DOCKER_BUILDKIT=1 docker build . -t "$(REPO):$(TAG)"

release:
	$(eval TAG=$(shell date +%Y%m%d))
	@$(MAKE) build TAG=$(TAG)
	@echo "Pushing $(REPO):$(TAG)"
	docker push "$(REPO):$(TAG)"
