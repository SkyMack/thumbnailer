all: clean build

build:
	go build -o thumbnailer ./cmd/thumbnailer

clean: clean-binary clean-outputs

clean-binary:
	rm ./thumbnailer

clean-outputs:
	rm -rf ./output/*
