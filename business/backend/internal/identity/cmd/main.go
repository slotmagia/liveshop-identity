// Command identity runs the module process.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lvtuopen-ai/kernel-go/logctx"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/app"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
)

func main() {
	path := flag.String("config", "", "path to the complete identity configuration file")
	flag.Parse()

	// Errors already name the module, so print them as-is.
	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string) error {
	settings, err := config.Load(path)
	if err != nil {
		return err
	}
	logctx.Configure(logctx.Options{Service: settings.Service, Level: settings.Log.Level, JSON: settings.Log.JSON})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dependencies, err := app.NewDependencies(ctx, settings)
	if err != nil {
		return err
	}
	defer func() { _ = dependencies.Close() }()

	httpServer := app.NewServer(dependencies)
	failures := make(chan error, 2)
	go func() { failures <- httpServer.Run(ctx) }()
	go func() { failures <- dependencies.GRPCServer.Serve() }()
	go dependencies.RegistrySync.Run(ctx)
	go dependencies.EntitlementSync.Run(ctx)
	select {
	case err := <-failures:
		stop()
		_ = dependencies.GRPCServer.Stop(context.Background())
		return err
	case <-ctx.Done():
		_ = dependencies.GRPCServer.Stop(context.Background())
		return <-failures
	}
}
