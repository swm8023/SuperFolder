package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"apphostdemo/service/backend"
	"github.com/gorilla/websocket"
)

func TestAppHandlerRegistersOnlyDemoMethodsOutsideBackendHandshake(t *testing.T) {
	handler := newAppHandler(true, 10*time.Millisecond)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, map[string]any{"id": 1, "method": backend.App.Hello, "payload": map[string]any{}})
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
	if helloPayload.App != appName || !helloPayload.Headless {
		t.Fatalf("hello payload = %+v", helloPayload)
	}

	writeJSON(t, conn, map[string]any{"id": 2, "method": backend.Demo.Ping, "payload": map[string]any{}})
	ping := readByID(t, conn, 2)
	if ping.Error != nil {
		t.Fatalf("ping error: %+v", ping.Error)
	}
	var pingPayload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(ping.Payload, &pingPayload); err != nil {
		t.Fatalf("decode ping payload: %v", err)
	}
	if pingPayload.Message != "pong" {
		t.Fatalf("ping payload = %+v", pingPayload)
	}

	tick := readNotification(t, conn, backend.Demo.Tick)
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

func readByID(t *testing.T, conn *websocket.Conn, id int) backend.Message {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var msg backend.Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if msg.ID != nil && *msg.ID == id {
			return msg
		}
	}
	t.Fatalf("message id %d not received", id)
	return backend.Message{}
}

func readNotification(t *testing.T, conn *websocket.Conn, method backend.Method) backend.Message {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		var msg backend.Message
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if msg.Method == method && msg.ID == nil {
			return msg
		}
	}
	t.Fatalf("notification %d not received", method)
	return backend.Message{}
}
