VERSION ?= dev
API_URL ?= http://localhost:4000
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.defaultAPIURL=$(API_URL)

.PHONY: build test vet snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o bin/coffre ./cmd/coffre

test:
	go test ./...

vet:
	go vet ./...

# Build multi-plateformes local sans publier (nécessite goreleaser)
snapshot:
	goreleaser release --snapshot --clean
