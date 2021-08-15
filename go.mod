module github.com/gjolly/filetransfer

go 1.16

require (
	github.com/grandcat/zeroconf v1.0.0
	github.com/miekg/dns v1.1.43 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0
	golang.org/x/net v0.0.0-20210726213435-c6fcb2dbf985 // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
)

replace github.com/grandcat/zeroconf v1.0.0 => github.com/gjolly/zeroconf v1.0.1-0.20210731103213-369a4a4953bc
