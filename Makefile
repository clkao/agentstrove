.PHONY: build frontend lint test test-store test-e2e test-all check serve install clean

build: frontend
	go build -o agentlore ./cmd/agentlore/

frontend:
	cd frontend && npm install && npm run build
	rm -rf internal/web/dist
	cp -r frontend/dist internal/web/dist

lint:
	golangci-lint run
	cd frontend && npx vitest run

test:
	go test ./internal/reader/... ./internal/secrets/... ./internal/gitlinks/... ./internal/sync/... -count=1 -v

test-store:
	go test ./internal/store/... -count=1 -v -timeout 60s

test-e2e:
	go test ./e2e/... -count=1 -v -timeout 120s

test-all: test test-store test-e2e

check: lint test-all

PREFIX ?= $(HOME)/.local
install: build
	install -d $(PREFIX)/bin
	install -m 755 agentlore $(PREFIX)/bin/agentlore

serve: build
	./agentlore serve

clean:
	rm -f agentlore
	rm -rf internal/web/dist
