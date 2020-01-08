COMMIT=$(shell git rev-list -1 HEAD)
DATE=$(shell date "+%Y-%m-%dT%H:%M:%S")
VERSION=$(shell git describe | cut -c 2-)

makes: ## builds makes
	echo "${COMMIT} ${DATE} ${VERSION}"
	go build -ldflags "-X main.commit=${COMMIT} -X main.date=${DATE} -X main.version=${VERSION}" -o makes ./cmd/makes/main.go
	cp ./makes ${GOPATH}/bin/makes

.PHONY: docker-build
docker-build: ## builds docker container from Dockerfile
	docker build -h

.PHONY: run-dev
run-dev: ## runs main.go continously during development
	gow run cmd/makes/main.go

dummy: echo ## Here goes the help string
	echo "dummy"

.PHONY: echo
echo: ## Test echo
	echo "Test"

.PHONY: clean
clean:
	rm makes