all: clean build

build: build/thumbnailer

build/thumbnailer:
	go build -o build/thumbnailer ./cmd/thumbnailer

clean: clean-binary clean-outputs

clean-binary:
	rm -f ./build/thumbnailer

clean-outputs:
	rm -rf ./output/*

PHONY: build