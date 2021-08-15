package := "./cmd/filetransfer"

all: build-linux build-windows build-macos

build-linux:
	GOOS=linux go build -o filetransfer.linux ${package}

build-windows:
	GOOS=windows go build ${package}

build-macos:
	GOOS=darwin go build -o filetransfer.macos ${package}

clean:
	rm filetransfer.exe filetransfer.linux filetransfer.macos
