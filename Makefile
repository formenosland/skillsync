.POSIX:

.PHONY: help
help:
	@printf '%s\n' \
		"targets:" \
		"  test           run sandboxed suite (tests/run.sh)" \
		"  shellcheck     lint shell scripts" \
		"  install        install via ./install.sh" \
		"  uninstall      remove tool only (install.sh --uninstall --yes)"

.PHONY: test
test:
	tests/run.sh

.PHONY: shellcheck
shellcheck:
	shellcheck -s sh bin/skillsync install.sh registry/generate.sh tests/run.sh

.PHONY: install
install:
	./install.sh

.PHONY: uninstall
uninstall:
	./install.sh --uninstall --yes
