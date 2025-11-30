GOCACHE ?= $(CURDIR)/.cache
BIN     ?= $(CURDIR)/bin/ktwins

.PHONY: run build clean

run:
	@echo ">> running ktwins"
	GOCACHE=$(GOCACHE) go run ./cmd/ktwins

build:
	@echo ">> building ktwins -> $(BIN)"
	@mkdir -p $(dir $(BIN))
	GOCACHE=$(GOCACHE) go build -o $(BIN) ./cmd/ktwins

clean:
	@echo ">> cleaning build artifacts"
	rm -rf $(BIN) $(CURDIR)/bin
	rm -rf $(CURDIR)/.cache
	go clean -cache
