.PHONY: wasm test-wasm test-workerd references

REFERENCES := plugins/awml/skills/awml/references
REFERENCE_EXAMPLES := client-email dinners question tour/09-platform-view

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

# The skill carries copies of the spec, the guide and some examples, each with its implementations'
# drawings, so it works without the repository.
references:
	mkdir -p $(REFERENCES)
	cp SPEC.md $(REFERENCES)/spec.md
	cp GUIDE.md $(REFERENCES)/guide.md
	for name in $(REFERENCE_EXAMPLES); do \
		mkdir -p "$(REFERENCES)/examples/$$(dirname $$name)"; \
		for file in examples/$$name.yaml examples/$$name.mmd examples/$$name-*.mmd; do \
			if test -f "$$file"; then cp "$$file" "$(REFERENCES)/$$file"; fi; \
		done; \
	done
