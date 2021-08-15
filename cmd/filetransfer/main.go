package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gjolly/filetransfer/pkg/encryption"
	"github.com/grandcat/zeroconf"
	bip39 "github.com/tyler-smith/go-bip39"
)

const maxNameSize = 100
const mDNSRecord = "_filetransfer._tcp"

// TODO: make that dynamic
const listenPort = 12345

func main() {
	log.SetFlags(0)

	mode := os.Args[1]
	arg := os.Args[2]

	if mode == "receive" || mode == "rcv" {
		receive(arg)
		return
	}

	if mode == "send" {
		send(arg)
		return
	}

	log.Fatalf("'%v' not supported. Supported modes: send, receive", arg)
}

func send(filePath string) {
	stat, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		log.Fatal("file doesn't exit")
	} else if err != nil {
		log.Fatal(err)
	} else if stat.IsDir() {
		log.Fatalf("%v is a folder, sending folders is not supported yet", filePath)
	}

	log.Println("Looking for peer on LAN, please start the program on the receiver side")

	serverAddr, err := locatePeer()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Receiver found at %v\n", serverAddr)

	conn, err := tls.Dial("tcp", serverAddr.String(), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	conn.Handshake()

	printPhrase(conn)

	_, fileName := path.Split(filePath)
	header := make([]byte, maxNameSize)
	for i, char := range []byte(fileName + "\x00") {
		header[i] = char
	}
	conn.Write([]byte(header))

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(conn, file)
	if err != nil {
		log.Fatal(err)
	}
}

func startAdvert(stop <-chan struct{}) {
	// Setup our service export
	host, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	server, err := zeroconf.Register(host, mDNSRecord, "local.", listenPort, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	<-stop
	defer server.Shutdown()
}

func locatePeer() (*net.TCPAddr, error) {
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	err = resolver.Browse(ctx, mDNSRecord, "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	timeout := time.NewTicker(5 * time.Second)
	for {
		select {
		case entry := <-entries:
			return &net.TCPAddr{
				IP:   entry.AddrIPv4[0],
				Port: entry.Port,
			}, nil
		case <-timeout.C:
			return nil, errors.New("no receiver found")
		}
	}
}

// receive is the server
func receive(destFolder string) {
	stat, err := os.Stat(destFolder)
	if err != nil && os.IsNotExist(err) {
		log.Fatal("The specified destination folder doesn't exit")
	} else if err != nil {
		log.Fatal(err)
	} else if !stat.IsDir() {
		log.Fatalf("%v is not a folder!", destFolder)
	}

	stop := make(chan struct{})
	go startAdvert(stop)

	pemCert, pemKey, err := encryption.GenerateCertificate()
	if err != nil {
		log.Fatalf("Fail to generate TLS certificate: %v", err)
	}

	cert, err := tls.X509KeyPair(pemCert, pemKey)
	if err != nil {
		log.Fatalf("Fail to generate TLS certificate: %v", err)
	}

	l, err := tls.Listen("tcp", fmt.Sprintf(":%v", listenPort), &tls.Config{
		Certificates: []tls.Certificate{
			cert,
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Waiting for sender")
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	tlsConn := conn.(*tls.Conn)
	tlsConn.Handshake()
	printPhrase(tlsConn)

	header := make([]byte, maxNameSize)
	n, err := conn.Read(header)
	if err != nil {
		log.Fatal(err)
	}
	if n < maxNameSize {
		log.Fatalf("Wrong header received %s", header)
	}

	var fileName string
	for _, char := range header {
		if char == 0x00 {
			break
		}
		fileName += string(char)
	}
	filePath := path.Join(destFolder, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Failed to create dest file: %v (%v chars)", err, len(filePath))
	}

	log.Printf("Receiving %v...\n", fileName)
	io.Copy(file, conn)

	stop <- struct{}{}
	file.Close()
	conn.Close()
	l.Close()

	log.Printf("File received at %v\n", filePath)
}

func printPhrase(conn *tls.Conn) {
	connState := conn.ConnectionState()
	exported, err := connState.ExportKeyingMaterial("filetransfer", nil, 16)
	if err != nil {
		log.Fatal(err)
	}

	mnemomic, err := bip39.NewMnemonic(exported)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("This is the secret for this exchange, make sure both side show the same sequence of words:")
	log.Println(mnemomic)

	log.Print("Continue [Y/n]? ")
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	text := strings.ToLower(input.Text())
	if len(text) != 0 && string(text[0]) != "y" {
		log.Fatal("wrong secret, filetransfer rejected")
	}
}
