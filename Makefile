.PHONY: build clean install-global install-python package-python

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.1")
INSTALL_DIR ?= $(HOME)/.local/share/forgectl/$(VERSION)/python

build:
	cd forgectl && go build -ldflags "-X forgectl/buildinfo.Version=$(VERSION)" -o forgectl .

install-python:
	@echo "Installing Python environment for forgectl $(VERSION)..."
	@command -v uv >/dev/null 2>&1 || { echo "Error: uv is required. Install it: https://docs.astral.sh/uv/getting-started/installation/"; exit 1; }
	mkdir -p $(INSTALL_DIR)
	cp reverse_engineer/pyproject.toml $(INSTALL_DIR)/
	cp reverse_engineer/uv.lock $(INSTALL_DIR)/
	cp reverse_engineer/.python-version $(INSTALL_DIR)/
	cp reverse_engineer/README.md $(INSTALL_DIR)/
	cp -r reverse_engineer/src $(INSTALL_DIR)/
	cd $(INSTALL_DIR) && uv sync --frozen --no-dev
	@echo "Generating checksums..."
	cd $(INSTALL_DIR) && \
		SHA_CMD=$$(command -v sha256sum 2>/dev/null || echo "shasum -a 256") && \
		find src -type f \( -name '*.py' -o -name '*.md' \) -exec $$SHA_CMD {} + > checksums.sha256
	@echo "Python environment installed at $(INSTALL_DIR)"

install-global: build install-python
	mkdir -p $(HOME)/.local/bin && cp forgectl/forgectl $(HOME)/.local/bin/forgectl
	@echo "forgectl $(VERSION) installed to $(HOME)/.local/bin/forgectl"

package-python:
	bash scripts/package-python.sh

clean:
	rm -f forgectl/forgectl
	rm -rf dist/
