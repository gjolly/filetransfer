package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"time"

	"github.com/grandcat/zeroconf"
)

const maxNameSize = 100
const mDNSRecord = "_filetransfer._tcp"

// TODO: make that dynamic
const listenPort = 12345

func main() {
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

	log.Println("looking for peer on LAN, please start the program on the receiver side")

	serverAddr, err := locatePeer()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("receiver found at %v\n", serverAddr)

	conn, err := net.Dial("tcp", serverAddr.String())
	if err != nil {
		log.Fatal(err)
	}

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

	entries := make(chan *zeroconf.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	err = resolver.Browse(ctx, mDNSRecord, "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()

	for entry := range entries {
		return &net.TCPAddr{
			IP:   entry.AddrIPv4[0],
			Port: entry.Port,
		}, nil
	}

	return nil, errors.New("No receiver found")
}

func receive(destFolder string) {
	stat, err := os.Stat(destFolder)
	if err != nil && os.IsNotExist(err) {
		log.Fatal("the specified destination folder doesn't exit")
	} else if err != nil {
		log.Fatal(err)
	} else if !stat.IsDir() {
		log.Fatalf("%v is not a folder!", destFolder)
	}

	stop := make(chan struct{})
	go startAdvert(stop)

	l, err := net.Listen("tcp", fmt.Sprintf(":%v", listenPort))
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("waiting for sender")
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}

	header := make([]byte, maxNameSize)
	n, err := conn.Read(header)
	if err != nil {
		log.Fatal(err)
	}
	if n < maxNameSize {
		log.Fatalf("wrong header received %s", header)
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

	log.Printf("receiving %v...\n", fileName)
	io.Copy(file, conn)

	stop <- struct{}{}
	file.Close()
	conn.Close()
	l.Close()

	log.Printf("file received at %v\n", filePath)
}
