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

func TestAppHandlerUsesSuperFolderIdentity(t *testing.T) {
	handler := newAppHandler(true)
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
	if helloPayload.App != "superfolder" || !helloPayload.Headless {
		t.Fatalf("hello payload = %+v", helloPayload)
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
