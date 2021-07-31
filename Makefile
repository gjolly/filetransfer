all: build-linux build-windows build-macos

build-linux:
	go build -o filetransfer.linux ./...

build-windows:
	GOOS=windows go build ./...

build-macos:
	GOOS=darwin go build -o filetransfer.macos ./...
