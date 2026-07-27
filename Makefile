.POSIX:

# Keep in sync with .github/workflows/ci.yml env.SHELLCHECK_VERSION
SHELLCHECK_VERSION = 0.11.0
SHELLCHECK_SRCS = bin/skillsync install.sh registry/generate.sh tests/run.sh

.PHONY: help
help:
	@printf '%s\n' \
		"targets:" \
		"  test         run sandboxed suite (tests/run.sh)" \
		"  shellcheck   lint with ShellCheck v$(SHELLCHECK_VERSION) + .shellcheckrc" \
		"  install      install via ./install.sh" \
		"  uninstall    remove tool only (install.sh --uninstall --yes)"

.PHONY: test
test:
	tests/run.sh

.PHONY: shellcheck
shellcheck:
	@command -v shellcheck >/dev/null || { \
		printf 'error: shellcheck not installed (want v%s)\n' "$(SHELLCHECK_VERSION)" >&2; \
		printf 'hint: brew install shellcheck   # or your package manager\n' >&2; \
		exit 1; \
	}
	@v=$$(shellcheck --version | awk '/^version:/{print $$2; exit}'); \
	[ "$$v" = "$(SHELLCHECK_VERSION)" ] || { \
		printf 'error: shellcheck %s, want %s (upgrade to match CI)\n' "$$v" "$(SHELLCHECK_VERSION)" >&2; \
		exit 1; \
	}
	shellcheck $(SHELLCHECK_SRCS)

.PHONY: install
install:
	./install.sh

.PHONY: uninstall
uninstall:
	./install.sh --uninstall --yes
