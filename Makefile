all: build-linux build-windows build-macos

build-linux:
	go build ./...

build-windows:
	GOOS=windows go build ./...

build-macos:
	GOOS=darwin go build -o filetransfer.macos ./...
