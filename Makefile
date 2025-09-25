EXAMPLES := $(notdir $(shell find examples -mindepth 1 -maxdepth 1 -type d))

.PHONY: $(EXAMPLES)
.PHONY: all
.PHONY: build
.PHONY: test
.PHONY: cleantest
.PHONY: vet
.PHONY: staticcheck
.PHONY: lint
.PHONY: clean
.PHONY: gosec

all: vet lint staticcheck test 

examples: $(EXAMPLES) 

$(EXAMPLES):
	@echo "*** building $@"
	@cd examples/$@ && CGO_ENABLED=0 go build -o ../../bin/$@

test:
	@echo "*** $@"
	@go test -timeout 2m ./...

cleantest:
	@echo "*** $@"
	@go clean -testcache

vet:
	@echo "*** $@"
	@go vet ./...

staticcheck:
	@echo "*** $@"
	@staticcheck ./...

lint:
	@echo "*** $@"
	@revive ./...

clean:
	@echo "*** cleaning binaries"
	@rm -rf bin

