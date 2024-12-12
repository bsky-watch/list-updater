.PHONY: all test

all: schema.json test

%.json: %.yaml
	@if ! which yq 2>&1 >/dev/null; then echo "Please install yq: sudo apt install yq"; exit 1; fi
	@yq --output-format=json . $< > $@

test:
	go test -v ./...
