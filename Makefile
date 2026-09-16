.POSIX:

GO = go
CGO_ENABLED = 0

.PHONY: all
all: build

.PHONY: help
help:
	@printf '%s\n' \
		"targets:" \
		"  all          build bin/skillsync (default)" \
		"  build        same as all" \
		"  test         go test ./..." \
		"  agentregistry  refresh internal/agentregistry/agents.tsv (SHA= for upstream commit)" \
		"  install      go install ./cmd/skillsync"

.PHONY: build
build:
	mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -o bin/skillsync ./cmd/skillsync

.PHONY: test
test: build
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./...

.PHONY: agentregistry
agentregistry:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) run ./internal/agentregistry/gen $(SHA)

.PHONY: install
install:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) install ./cmd/skillsync
