package main

import (
	"fmt"
	"os"

	"apphostdemo/service/backend"
	"apphostdemo/service/superfolder"
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
	return newAppHandlerWithOptions(headless, superfolder.Options{})
}

func newAppHandlerWithOptions(headless bool, options superfolder.Options) *backend.Server {
	app, err := superfolder.NewApp(options)
	if err != nil {
		panic(err)
	}
	handler := backend.NewServer(backend.ServerOptions{
		AppName:  appName,
		Headless: headless,
	})
	app.Register(handler)
	return handler
}
