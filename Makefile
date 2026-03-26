PLUGIN_DIR := ~/.tflint.d/plugins
BINARY     := tflint-ruleset-jeremad

.PHONY: build test coverage install

build:
	go build -o $(BINARY) .

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/$(BINARY)
