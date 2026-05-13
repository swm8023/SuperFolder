package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClassifyMessage(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want MessageKind
	}{
		{"call", `{"id":1,"method":2000001,"payload":{}}`, MessageCall},
		{"success", `{"id":1,"payload":{"message":"pong"}}`, MessageSuccess},
		{"failure", `{"id":1,"error":{"code":1001,"message":"missing"}}`, MessageFailure},
		{"notification", `{"method":2000002,"payload":{"count":1}}`, MessageNotification},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := DecodeMessage([]byte(tc.raw))
			if err != nil {
				t.Fatalf("DecodeMessage returned error: %v", err)
			}
			if got := ClassifyMessage(msg); got != tc.want {
				t.Fatalf("ClassifyMessage() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeadlessRequiresExplicitPort(t *testing.T) {
	err := Run([]string{"--headless"}, HostOptions{
		AppName: "app-host-demo",
		NewHandler: func(headless bool) http.Handler {
			t.Fatal("handler should not be created when headless port is missing")
			return nil
		},
		LaunchUI: func(string) error {
			t.Fatal("launcher should not be called in headless mode")
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "--headless requires --port") {
		t.Fatalf("Run returned %v, want headless port error", err)
	}
}

func TestNonHeadlessLaunchesNativeUIAndReturnsWhenWindowCloses(t *testing.T) {
	var launchedURL string
	err := Run([]string{"--port", "0"}, HostOptions{
		AppName: "app-host-demo",
		NewHandler: func(headless bool) http.Handler {
			return NewServer(ServerOptions{AppName: "app-host-demo", Headless: headless})
		},
		LaunchUI: func(url string) error {
			launchedURL = url
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.HasPrefix(launchedURL, "http://127.0.0.1:") {
		t.Fatalf("launcher URL = %q", launchedURL)
	}
}

func TestClassifyMessageRejectsInvalidShapes(t *testing.T) {
	cases := []string{
		`{"id":0,"method":2000001,"payload":{}}`,
		`{"id":1,"method":2000001}`,
		`{"method":2000002}`,
		`{"id":1,"payload":{},"error":{"code":1001,"message":"bad"}}`,
	}

	for _, raw := range cases {
		msg, err := DecodeMessage([]byte(raw))
		if err != nil {
			t.Fatalf("DecodeMessage returned error: %v", err)
		}
		if got := ClassifyMessage(msg); got != MessageInvalid {
			t.Fatalf("ClassifyMessage(%s) = %v, want %v", raw, got, MessageInvalid)
		}
	}

	if _, err := DecodeMessage([]byte(`{"id":1,"method":"demo.ping","payload":{}}`)); err == nil {
		t.Fatal("DecodeMessage accepted string method, want error")
	}
}

func TestHealthAndBootHandlers(t *testing.T) {
	server := httptest.NewServer(NewServer(ServerOptions{AppName: "app-host-demo", Headless: true}))
	defer server.Close()

	healthResp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer healthResp.Body.Close()

	var health struct {
		OK  bool   `json:"ok"`
		App string `json:"app"`
	}
	if err := json.NewDecoder(healthResp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if !health.OK || health.App != "app-host-demo" {
		t.Fatalf("health = %+v", health)
	}

	bootResp, err := http.Get(server.URL + "/boot")
	if err != nil {
		t.Fatalf("GET /boot failed: %v", err)
	}
	defer bootResp.Body.Close()

	var boot struct {
		App      string `json:"app"`
		Headless bool   `json:"headless"`
		RPCURL   string `json:"rpcUrl"`
	}
	if err := json.NewDecoder(bootResp.Body).Decode(&boot); err != nil {
		t.Fatalf("decode boot response: %v", err)
	}
	if boot.App != "app-host-demo" || !boot.Headless {
		t.Fatalf("boot = %+v", boot)
	}
	if !strings.HasPrefix(boot.RPCURL, "ws://") || !strings.HasSuffix(boot.RPCURL, "/ws") {
		t.Fatalf("rpcUrl = %q", boot.RPCURL)
	}
}

func TestServerHandlesAppHelloAndRunsSessionReadyHook(t *testing.T) {
	handler := NewServer(ServerOptions{
		AppName:  "app-host-demo",
		Headless: true,
		OnSessionReady: func(ctx CallContext) {
			ctx.StartSessionTask("test.tick", func(taskCtx context.Context, notify NotifyFunc) {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				select {
				case <-taskCtx.Done():
					return
				case <-ticker.C:
					_ = notify(Demo.Tick, map[string]any{"count": 1, "message": "tick"})
				}
			})
		},
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, map[string]any{"id": 1, "method": App.Hello, "payload": map[string]any{}})
	hello := readByID(t, conn, 1)
	if hello.Error != nil {
		t.Fatalf("hello error: %+v", hello.Error)
	}
	var helloPayload struct {
		App      string `json:"app"`
		Headless bool   `json:"headless"`
	}
	if err := json.Unmarshal(hello.Payload, &helloPayload); err != nil {
		t.Fatalf("decode hello payload: %v", err)
	}
	if helloPayload.App != "app-host-demo" || !helloPayload.Headless {
		t.Fatalf("hello payload = %+v", helloPayload)
	}

	tick := readNotification(t, conn, Demo.Tick)
	var tickPayload struct {
		Count   int    `json:"count"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(tick.Payload, &tickPayload); err != nil {
		t.Fatalf("decode tick payload: %v", err)
	}
	if tickPayload.Count != 1 || tickPayload.Message != "tick" {
		t.Fatalf("tick payload = %+v", tickPayload)
	}
}

func TestServerAllowsRegisteringHandlersAndSessionTasksByGeneratedMethod(t *testing.T) {
	handler := NewServer(ServerOptions{
		AppName:  "app-host-demo",
		Headless: true,
	})
	handler.RegisterHandler(Demo.Ping, func(ctx CallContext) (any, *RPCError) {
		ctx.StartSessionTask("test.tick", func(taskCtx context.Context, notify NotifyFunc) {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				_ = notify(Demo.Tick, map[string]any{"count": 1, "message": "tick"})
			}
		})
		return map[string]any{"message": "custom"}, nil
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, map[string]any{"id": 1, "method": Demo.Ping, "payload": map[string]any{}})
	ping := readByID(t, conn, 1)
	if ping.Error != nil {
		t.Fatalf("ping error: %+v", ping.Error)
	}
	var pingPayload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(ping.Payload, &pingPayload); err != nil {
		t.Fatalf("decode ping payload: %v", err)
	}
	if pingPayload.Message != "custom" {
		t.Fatalf("ping payload = %+v", pingPayload)
	}

	tick := readNotification(t, conn, Demo.Tick)
	var tickPayload struct {
		Count   int    `json:"count"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(tick.Payload, &tickPayload); err != nil {
		t.Fatalf("decode tick payload: %v", err)
	}
	if tickPayload.Count != 1 || tickPayload.Message != "tick" {
		t.Fatalf("tick payload = %+v", tickPayload)
	}
}

func writeJSON(t *testing.T, conn *websocket.Conn, value any) {
	t.Helper()
	if err := conn.WriteJSON(value); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
}

func readByID(t *testing.T, conn *websocket.Conn, id int) Message {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if msg.ID != nil && *msg.ID == id {
			return msg
		}
	}
	t.Fatalf("message id %d not received", id)
	return Message{}
}

func readNotification(t *testing.T, conn *websocket.Conn, method Method) Message {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if msg.Method == method && msg.ID == nil {
			return msg
		}
	}
	t.Fatalf("notification %d not received", method)
	return Message{}
}
