.PHONY: build install test e2e fmt lint install-hooks clean

build:
	mkdir -p bin
	go build -o bin/tkt .

install:
	go install .

test:
	go test ./...

fmt:
	gofmt -w .
	@which goimports > /dev/null 2>&1 && goimports -w . || true

lint:
	golangci-lint run ./...

e2e: build
	go test -tags e2e -v ./e2e/...

install-hooks:
	ln -sf ../../.githooks/pre-push .git/hooks/pre-push
	@echo "pre-push hook installed"

clean:
	rm -f bin/tkt
