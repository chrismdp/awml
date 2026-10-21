.PHONY: wasm test-wasm test-workerd

AWML_GO := $(wildcard *.go) $(wildcard cmd/awml-wasm/*.go) go.mod go.sum

wasm: build/awml.wasm build/wasm_exec.js

build/awml.wasm: $(AWML_GO)
	mkdir -p build
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o $@ ./cmd/awml-wasm
	gzip -9 -c $@ > $@.gz

build/wasm_exec.js:
	mkdir -p build
	shim="$$(go env GOROOT)/lib/wasm/wasm_exec.js"; \
	if test ! -f "$$shim"; then shim="$$(go env GOROOT)/misc/wasm/wasm_exec.js"; fi; \
	cp "$$shim" $@

test-wasm: wasm
	go build -o build/awml-native ./cmd/awml
	node wasm/parity.test.mjs

test-workerd: wasm
	bash scripts/test-workerd.sh
