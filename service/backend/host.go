package backend

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

type Handler = http.Handler

type HandlerFactory func(headless bool) Handler

type HostOptions struct {
	AppName     string
	WindowTitle string
	NewHandler  HandlerFactory
	LaunchUI    func(url string) error
}

type NativeWindowOptions struct {
	Title   string
	Width   int
	Height  int
	AppName string
}

func Run(args []string, options HostOptions) error {
	if options.NewHandler == nil {
		return fmt.Errorf("handler factory is required")
	}

	appName := options.AppName
	if appName == "" {
		appName = "app"
	}

	flags := flag.NewFlagSet(appName, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	headless := flags.Bool("headless", false, "run service without opening UI")
	port := flags.Int("port", 0, "service port")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *headless && *port == 0 {
		return fmt.Errorf("--headless requires --port <int>")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("unexpected listener address: %s", listener.Addr().String())
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", tcpAddr.Port)
	rpcURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", tcpAddr.Port)
	log.Printf("%s listening on %s", appName, url)
	log.Printf("headless=%t", *headless)
	log.Printf("rpc=%s", rpcURL)

	server := &http.Server{Handler: options.NewHandler(*headless)}
	if *headless {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	default:
	}

	launchUI := options.LaunchUI
	if launchUI == nil {
		windowTitle := options.WindowTitle
		if windowTitle == "" {
			windowTitle = appName
		}
		launchUI = func(url string) error {
			return LaunchWebView(url, NativeWindowOptions{
				Title:   windowTitle,
				Width:   1200,
				Height:  800,
				AppName: appName,
			})
		}
	}

	launchErr := launchUI(url)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	if err := <-serverErr; err != nil {
		return err
	}
	if launchErr != nil {
		return launchErr
	}
	return nil
}

func LaunchWebView(url string, options NativeWindowOptions) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("create WebView2 window failed: %v", recovered)
		}
	}()

	dataPath, err := webViewDataPath(options.AppName)
	if err != nil {
		return err
	}

	title := options.Title
	if title == "" {
		title = options.AppName
	}
	if title == "" {
		title = "APP"
	}

	width := options.Width
	if width <= 0 {
		width = 1200
	}

	height := options.Height
	if height <= 0 {
		height = 800
	}

	view := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		DataPath:  dataPath,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  uint(width),
			Height: uint(height),
			Center: true,
		},
	})
	if view == nil {
		return fmt.Errorf("create WebView2 window failed")
	}
	defer view.Destroy()

	view.Navigate(url)
	view.Run()
	return nil
}

func webViewDataPath(appName string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir failed: %w", err)
	}
	if appName == "" {
		appName = "app"
	}
	return filepath.Join(cacheDir, "SuperFolder", appName, "webview2"), nil
}
