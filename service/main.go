package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"apphostdemo/service/backend"
)

const appName = "app-host-demo"
const defaultDemoTickInterval = 2 * time.Second

func main() {
	if err := backend.Run(os.Args[1:], backend.HostOptions{
		AppName:     appName,
		WindowTitle: "APP Host Demo",
		NewHandler: func(headless bool) backend.Handler {
			return newAppHandler(headless, defaultDemoTickInterval)
		},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newAppHandler(headless bool, tickInterval time.Duration) *backend.Server {
	if tickInterval <= 0 {
		tickInterval = defaultDemoTickInterval
	}

	handler := backend.NewServer(backend.ServerOptions{
		AppName:  appName,
		Headless: headless,
		OnSessionReady: func(ctx backend.CallContext) {
			startDemoTick(ctx, tickInterval)
		},
	})
	registerDemoHandlers(handler)
	return handler
}

func registerDemoHandlers(handler *backend.Server) {
	handler.RegisterHandler(backend.Demo.Ping, func(ctx backend.CallContext) (any, *backend.RPCError) {
		return map[string]any{"message": "pong"}, nil
	})
}

func startDemoTick(ctx backend.CallContext, tickInterval time.Duration) {
	ctx.StartSessionTask("demo.tick", func(taskCtx context.Context, notify backend.NotifyFunc) {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		count := 0
		for {
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				count++
				if err := notify(backend.Demo.Tick, map[string]any{
					"count":   count,
					"message": "tick",
				}); err != nil {
					return
				}
			}
		}
	})
}
