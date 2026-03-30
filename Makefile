.PHONY: build clean install-global

build:
	cd forgectl && go build -o forgectl .

clean:
	rm -f forgectl/forgectl

install-global: build
	mkdir -p ~/.local/bin && cp forgectl/forgectl ~/.local/bin/forgectl