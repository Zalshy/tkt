.PHONY: build install test e2e fmt lint clean

build:
	mkdir -p bin
	go build -o bin/tkt .

install:
	go install .

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	go vet ./...

e2e: build
	go test -tags e2e -v ./e2e/...

clean:
	rm -f bin/tkt
