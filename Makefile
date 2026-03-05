.PHONY: build frontend test test-store test-e2e test-all serve clean

build: frontend
	go build -o agentstrove ./cmd/agentstrove/

frontend:
	cd frontend && npm install && npm run build
	rm -rf internal/web/dist
	cp -r frontend/dist internal/web/dist

test:
	go test ./internal/reader/... ./internal/secrets/... ./internal/gitlinks/... ./internal/sync/... -count=1 -v

test-store:
	go test ./internal/store/... -count=1 -v -timeout 60s

test-e2e:
	go test ./e2e/... -count=1 -v -timeout 120s

test-all: test test-store test-e2e

serve: build
	./agentstrove serve

clean:
	rm -f agentstrove
	rm -rf internal/web/dist
