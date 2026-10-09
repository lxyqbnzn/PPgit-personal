package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ppgit-go/internal/core"
	"ppgit-go/internal/launcher"
	"ppgit-go/internal/platform"
	"ppgit-go/internal/server"
)

var version = "1.1.0"

type instance = launcher.Instance

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func randomID() (string, error) {
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	return hex.EncodeToString(raw), err
}

func main() {
	if err := run(); err != nil {
		log.Print(err)
		if platform.IsAddressInUse(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	data := flag.String("data", launcher.DataDir(executable), "PPGit data directory")
	host := flag.String("host", env("PPGIT_HOST", "127.0.0.1"), "Bind host (local use only)")
	port := flag.String("port", env("PPGIT_PORT", "9300"), "HTTP port; 0 selects a free port")
	fallbackPort := flag.Bool("fallback-port", false, "Use an available port when the preferred port is occupied")
	noBrowser := flag.Bool("no-browser", os.Getenv("PPGIT_NO_BROWSER") == "1", "Do not open the default browser")
	stop := flag.Bool("stop", false, "Stop only the verified instance using this data directory")
	stopPortOwner := flag.Bool("stop-port-owner", false, "Stop a verified PPGit installation using the preferred loopback port (Windows)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("PPGit Go " + version)
		return nil
	}
	*data, err = filepath.Abs(*data)
	if err != nil {
		return err
	}
	runtimePath := filepath.Join(*data, "runtime.json")
	if *stop {
		return stopInstance(runtimePath)
	}
	if *stopPortOwner {
		stopped, err := launcher.StopPortOwner(*host, *port)
		if err != nil {
			return err
		}
		if stopped {
			fmt.Println("PPGit Go shutdown requested; wait for active operations to finish before starting again.")
		} else {
			fmt.Println("No running PPGit Go instance was found on the preferred port. There is nothing to stop.")
		}
		return nil
	}
	if err := os.MkdirAll(*data, 0700); err != nil {
		return err
	}
	unlock, err := platform.Lock(filepath.Join(*data, "instance.lock"))
	if err != nil {

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			old, verifyErr := readInstance(runtimePath)
			if verifyErr == nil {
				fmt.Println(old.URL)
				if !*noBrowser {
					_ = platform.OpenBrowser(old.URL)
				}
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return err
	}
	defer unlock()
	logFile, err := os.OpenFile(filepath.Join(*data, "ppgit.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	engine, err := core.New(*data)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(strings.Trim(*host, "[]"), *port))
	if err != nil && *fallbackPort && platform.IsAddressInUse(err) {
		listener, err = net.Listen("tcp", net.JoinHostPort(strings.Trim(*host, "[]"), "0"))
	}
	if err != nil {
		return fmt.Errorf("cannot bind server (choose another -port): %w", err)
	}
	defer listener.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	id := os.Getenv("PPGIT_INSTANCE_ID")
	if id == "" {
		id, err = randomID()
		if err != nil {
			return err
		}
	}
	shutdownKey, err := randomID()
	if err != nil {
		return err
	}
	handler, err := server.New(engine, server.Options{
		Host: *host, TrustedHosts: strings.Split(os.Getenv("PPGIT_TRUSTED_HOSTS"), ","), InstanceID: id,
		BrowseFolder: func() (string, error) { return platform.BrowseFolderContext(ctx) }, Shutdown: cancel, ShutdownToken: shutdownKey,
	})
	if err != nil {
		return err
	}
	browserHost := strings.Trim(*host, "[]")
	if browserHost == "0.0.0.0" {
		browserHost = "127.0.0.1"
	}
	if browserHost == "::" {
		browserHost = "::1"
	}
	if ip, err := netip.ParseAddr(browserHost); browserHost != "localhost" && (err != nil || !ip.IsLoopback()) {
		log.Print("WARNING: PPGit is a local single-user tool, not an authenticated remote server.")
	}
	address := "http://" + net.JoinHostPort(browserHost, fmt.Sprint(listener.Addr().(*net.TCPAddr).Port))
	info := instance{URL: address, ID: id, Token: handler.Token(), ShutdownToken: shutdownKey, PID: os.Getpid()}
	payload, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(runtimePath, payload, 0600); err != nil {
		return err
	}
	defer os.Remove(runtimePath)
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 * 1024}
	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(listener) }()
	log.Printf("PPGit Go %s ready at %s (data: %s)", version, address, *data)
	if !*noBrowser {
		if err := platform.OpenBrowser(address); err != nil {
			log.Printf("Open the browser manually: %s", address)
		}
	}
	var serveErr error
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	case <-ctx.Done():
	}
	cancel()
	log.Print("Shutting down; waiting for active requests and background scans.")
	return errors.Join(serveErr, drainServer(httpServer, engine))
}

func drainServer(httpServer *http.Server, engine *core.Core) error {
	requestErr := httpServer.Shutdown(context.Background())
	return errors.Join(requestErr, engine.Close())
}

func readInstance(path string) (instance, error) {
	return launcher.Read(path)
}

func stopInstance(path string) error {
	if err := launcher.Stop(path); err != nil {
		return err
	}
	fmt.Println("PPGit Go shutdown requested.")
	return nil
}
