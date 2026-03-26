// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apis "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/APIs"
	app "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app"
	bootstrap "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/bootstrap"
)

const (
	apiAddr         = ":9090"
	shutdownTimeout = 15 * time.Second
)

type engineRuntime struct {
	deps app.Dependencies
}

func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err.Error())
	}
}

func run() error {
	manager := apis.NewSessionManager(buildEngineRuntime)
	keys, err := loadAPIKeyPair()
	if err != nil {
		return err
	}
	router, err := apis.NewRouter(manager, keys, apis.DefaultEndpointContractPath)
	if err != nil {
		return err
	}
	fmt.Printf("Infermal API public key: %s\n", keys.Public)
	server := newHTTPServer(apiAddr, router)
	return serveUntilShutdown(server, manager)
}

func loadAPIKeyPair() (apis.APIKeyPair, error) {
	privateKey := os.Getenv(apis.PrivateKeyEnv())
	privateKeyPath := os.Getenv(apis.PrivateKeyPathEnv())
	return apis.LoadOrCreateAPIKeyPair(privateKey, privateKeyPath)
}

func buildEngineRuntime() (apis.Runtime, error) {
	deps, err := bootstrap.BuildEngineDependencies()
	if err != nil {
		return nil, err
	}
	return engineRuntime{deps: deps}, nil
}

func (r engineRuntime) Run(ctx context.Context) error {
	return app.Run(ctx, r.deps)
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func serveUntilShutdown(server *http.Server, manager *apis.SessionManager) error {
	errCh := make(chan error, 1)
	go startHTTPServer(server, errCh)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case err := <-errCh:
		manager.Shutdown(shutdownTimeout)
		return err
	case sig := <-signalCh:
		fmt.Println("Signal received:", sig.String())
		return shutdownServer(server, manager)
	}
}

func startHTTPServer(server *http.Server, errCh chan<- error) {
	fmt.Printf("Infermal API listening on %s\n", server.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		errCh <- nil
		return
	}
	errCh <- err
}

func shutdownServer(server *http.Server, manager *apis.SessionManager) error {
	manager.Shutdown(shutdownTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Shutdown complete.")
	return nil
}
