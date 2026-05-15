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
		AppName: "test-app",
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
		AppName: "test-app",
		NewHandler: func(headless bool) http.Handler {
			return NewServer(ServerOptions{AppName: "test-app", Headless: headless})
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

	if _, err := DecodeMessage([]byte(`{"id":1,"method":"folder.session.get","payload":{}}`)); err == nil {
		t.Fatal("DecodeMessage accepted string method, want error")
	}
}

func TestHealthAndBootHandlers(t *testing.T) {
	server := httptest.NewServer(NewServer(ServerOptions{AppName: "test-app", Headless: true}))
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
	if !health.OK || health.App != "test-app" {
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
	if boot.App != "test-app" || !boot.Headless {
		t.Fatalf("boot = %+v", boot)
	}
	if !strings.HasPrefix(boot.RPCURL, "ws://") || !strings.HasSuffix(boot.RPCURL, "/ws") {
		t.Fatalf("rpcUrl = %q", boot.RPCURL)
	}
}

func TestServerHandlesAppHelloAndRunsSessionReadyHook(t *testing.T) {
	handler := NewServer(ServerOptions{
		AppName:  "test-app",
		Headless: true,
		OnSessionReady: func(ctx CallContext) {
			ctx.StartSessionTask("test.children.updated", func(taskCtx context.Context, notify NotifyFunc) {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				select {
				case <-taskCtx.Done():
					return
				case <-ticker.C:
					_ = notify(Folder.Children.Updated, map[string]any{"path": "C:\\tmp"})
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
	if helloPayload.App != "test-app" || !helloPayload.Headless {
		t.Fatalf("hello payload = %+v", helloPayload)
	}

	update := readNotification(t, conn, Folder.Children.Updated)
	var updatePayload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(update.Payload, &updatePayload); err != nil {
		t.Fatalf("decode update payload: %v", err)
	}
	if updatePayload.Path != "C:\\tmp" {
		t.Fatalf("update payload = %+v", updatePayload)
	}
}

func TestServerAllowsRegisteringHandlersAndSessionTasksByGeneratedMethod(t *testing.T) {
	handler := NewServer(ServerOptions{
		AppName:  "test-app",
		Headless: true,
	})
	handler.RegisterHandler(Folder.Session.Get, func(ctx CallContext) (any, *RPCError) {
		ctx.StartSessionTask("test.children.updated", func(taskCtx context.Context, notify NotifyFunc) {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			select {
			case <-taskCtx.Done():
				return
			case <-ticker.C:
				_ = notify(Folder.Children.Updated, map[string]any{"path": "C:\\tmp"})
			}
		})
		return map[string]any{"session": "custom"}, nil
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, map[string]any{"id": 1, "method": Folder.Session.Get, "payload": map[string]any{}})
	session := readByID(t, conn, 1)
	if session.Error != nil {
		t.Fatalf("session error: %+v", session.Error)
	}
	var sessionPayload struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal(session.Payload, &sessionPayload); err != nil {
		t.Fatalf("decode session payload: %v", err)
	}
	if sessionPayload.Session != "custom" {
		t.Fatalf("session payload = %+v", sessionPayload)
	}

	update := readNotification(t, conn, Folder.Children.Updated)
	var updatePayload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(update.Payload, &updatePayload); err != nil {
		t.Fatalf("decode update payload: %v", err)
	}
	if updatePayload.Path != "C:\\tmp" {
		t.Fatalf("update payload = %+v", updatePayload)
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
