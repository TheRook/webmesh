package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/webmeshproj/webmesh/pkg/campfire"
	"github.com/webmeshproj/webmesh/pkg/util"
)

func main() {
	fmt.Println(len(os.Args), os.Args)
	campURI := flag.String("camp", "camp://turn?fingerprint#psk", "camp URI")
	logLevel := flag.String("log-level", "info", "log level")
	flag.Parse()
	log := util.SetupLogging(*logLevel)
	if *campURI == "" {
		fmt.Fprintln(os.Stderr, "a Camp URL is required")
		os.Exit(1)
	}
	fmt.Println("Conneting to:", campURI)
	ctx := context.Background()
	ourcamp, err := campfire.ParseCampfireURI(*campURI)
	if err != nil {
		fmt.Fprintln(os.Stderr, "a Camp URL is required", err)
		os.Exit(1)
	}

	//Wait at a specific campfire:
	cf, err := campfire.Wait(ctx, ourcamp)

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer cf.Close()

	go func() {
		select {
		case err := <-cf.Errors():
			log.Error("error", "error", err.Error())
			os.Exit(1)
		case <-cf.Expired():
			log.Info("campfire expired")
			os.Exit(0)
		}
	}()

	fmt.Println(">>> Waiting for connections")
	conn, err := cf.Accept()
	if err != nil {
		log.Error("error", "error", err.Error())
		return
	}
	fmt.Println(">>> New peer connection")
	go func() {
		defer conn.Close()
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				log.Error("error", "error", err.Error())
				return
			}
			fmt.Println("remote:", string(buf[:n]))
			fmt.Print("> ")
		}
	}()
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := in.ReadBytes('\n')
		if err != nil {
			log.Error("error", "error", err.Error())
			return
		}
		_, err = conn.Write(bytes.TrimSpace(line))
		if err != nil {
			log.Error("error", "error", err.Error())
			return
		}
	}
}
