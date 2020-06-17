package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	//"log"
	//"os/exec"
	//"strconv"
	"context"
	"log"
	"net/http"
	"os/exec"
	"os/signal"
	"os/user"
	"time"
)

// Variables
var (
	certCheckOk = false
	certDir     = "/tmp/cert"

	host       = flag.String("host", "", "Comma-separated hostnames and IPs to generate a certificate for")
	validFrom  = flag.String("start-date", "", "Creation date formatted as Jan 1 15:04:05 2011")
	validFor   = flag.Duration("duration", 365*24*time.Hour, "Duration that certificate is valid for")
	isCA       = flag.Bool("ca", false, "whether this cert should be its own Certificate Authority")
	rsaBits    = flag.Int("rsa-bits", 2048, "Size of RSA key to generate. Ignored if --ecdsa-curve is set")
	ecdsaCurve = flag.String("ecdsa-curve", "", "ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521")
	ed25519Key = flag.Bool("ed25519", false, "Generate an Ed25519 key")
	listenAddr = flag.String("listen-addr", "", "Server listen address")

	listenAddrhttp = flag.String("enable-http", "", "Server listen address for http")
	///hostname string
)

func main() {
	if runtime.GOOS != "linux" {
		fmt.Println("This program is only works on linux devices")
		return
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	if user.Name != "www-data" {
		fmt.Printf("To secure your system, please run this program as www-data."+
			"Current user %v \n", user.Name)
		os.Exit(3)
	}

	requiredPrograms := []string{"ping", "traceroute", "whois", "nslookup"}
	var requiredapps [4]bool
	for i, s := range requiredPrograms {
		// Get path of required programs
		_, err := exec.LookPath(s)
		if err == nil {
			//fmt.Printf("Required program %v %v found at %v\n", i+1, s, path)
			requiredapps[i] = true //save to array
		} else {
			fmt.Printf("Required program %v : %v cannot found.\n", i+1, s)
			requiredapps[i] = false
			if i < 4 { //sh and df is must required. If is not found in software than exit.
				fmt.Printf("Please install %v and run this program again\n", s)
				os.Exit(3)
			}
		}
	}

	flag.Bool("help", false, "")
	flag.Bool("h", false, "")
	flag.Usage = func() {}

	certCheck()

	//args := flag.Args()
	if *listenAddr == "" && *listenAddrhttp == "" {
		fmt.Println("no http or https listen address given")
		var tmpaddr = ":443"
		listenAddr = &tmpaddr
	}
	flag.Parse()

	logger := log.New(os.Stdout, "https: ", log.LstdFlags)
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	server := webServer(logger)
	go gracefullShutdown(server, logger, quit, done)
	logger.Println("Server is ready to handle requests at", *listenAddr)
	if err := server.ListenAndServeTLS(certDir+"/cert.pem", certDir+"/key.pem"); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", *listenAddr, err)
	}
	<-done
	logger.Println("Server stopped")

	fmt.Printf("Program Closed.")
}

var help = `
it will be written
Read more:
https://github.com/ahmetozer/looking-glass-controller
`

func certCheck() {
	if _, err := os.Stat("/cert/key.pem"); err == nil {
		fmt.Printf("/cert/key.pem exists\n")
		certCheckOk = true
	} else {
		fmt.Printf("/cert/key.pem not exist\n")
		certCheckOk = false
	}
	if certCheckOk == true {
		if _, err := os.Stat("/cert/cert.pem"); err == nil {
			fmt.Printf("/cert/cert.pem exists\n")
			certCheckOk = true
		} else {
			fmt.Printf("/cert/cert.pem not exist\n")
			certCheckOk = false
		}
	}

	if certCheckOk == true {
		certDir = "/cert"
	} else {
		fmt.Printf("Self certs will be used\n")
		certDir = "/tmp/cert"
		sslCertGenerate()
	}

}

//Grace Full Shutdown
func gracefullShutdown(server *http.Server, logger *log.Logger, quit <-chan os.Signal, done chan<- bool) {
	<-quit
	logger.Println("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
	}
	close(done)
}