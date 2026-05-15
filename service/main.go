package main

import (
	"fmt"
	"os"

	"apphostdemo/service/backend"
)

const appName = "superfolder"

func main() {
	if err := backend.Run(os.Args[1:], backend.HostOptions{
		AppName:     appName,
		WindowTitle: "SuperFolder",
		NewHandler: func(headless bool) backend.Handler {
			return newAppHandler(headless)
		},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newAppHandler(headless bool) *backend.Server {
	handler := backend.NewServer(backend.ServerOptions{
		AppName:  appName,
		Headless: headless,
	})
	return handler
}
