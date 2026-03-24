PLUGIN_DIR := ~/.tflint.d/plugins
BINARY     := tflint-ruleset-jeremad

.PHONY: build test install

build:
	go build -o $(BINARY) .

test:
	go test ./...

install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/$(BINARY)
