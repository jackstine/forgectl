.PHONY: build clean install-global test test-integration

build:
	cd forgectl && go build -o forgectl .

# Fast in-process unit suite.
test:
	cd forgectl && go test ./...

# Black-box integration suite: builds the real binary and drives it end-to-end
# over a real filesystem and git. Gated behind the `integration` build tag so it
# stays out of the fast unit run. See forgectl/integration/README.md.
test-integration:
	cd forgectl && go test -tags=integration ./integration/...

clean:
	rm -f forgectl/forgectl

install-global: build
	mkdir -p ~/.local/bin && cp forgectl/forgectl ~/.local/bin/forgectl