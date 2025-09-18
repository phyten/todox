BIN=bin/todox

.PHONY: build run serve clean fmt vet

build:
	mkdir -p bin
	go build -o $(BIN) ./cmd/todox

run: build
	$(BIN)

serve: build
	$(BIN) serve -p 8080

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin
