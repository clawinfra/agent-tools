.PHONY: build test coverage lint dev-setup dev clean proto

BINARY     := agent-tools
MAIN       := ./cmd/agent-tools
COVERAGE   := coverage.out
THRESHOLD  := 90
TAGS       := sqlite_fts5

build:
	CGO_ENABLED=1 go build -tags $(TAGS) -ldflags="-s -w" -o $(BINARY) $(MAIN)

test:
	CGO_ENABLED=1 go test -v -race -tags $(TAGS) ./...

coverage:
	CGO_ENABLED=1 go test -race -tags $(TAGS) -coverprofile=$(COVERAGE) -covermode=atomic ./...
	@go tool cover -func=$(COVERAGE) | grep total
	@COVERAGE=$$(go tool cover -func=$(COVERAGE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$COVERAGE < $(THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage $$COVERAGE% is below the required $(THRESHOLD)%"; \
		exit 1; \
	fi; \
	echo "✅ Coverage $$COVERAGE% meets the $(THRESHOLD)% threshold"

coverage-html: coverage
	go tool cover -html=$(COVERAGE) -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run --timeout=5m

vet:
	go vet -tags $(TAGS) ./...

dev-setup:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go mod download

dev:
	air -c .air.toml

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto

clean:
	rm -f $(BINARY) $(COVERAGE) coverage.html

docker-build:
	podman build -t ghcr.io/clawinfra/agent-tools:dev .

docker-run:
	podman run -p 8433:8433 -v agent-tools-data:/data ghcr.io/clawinfra/agent-tools:dev
