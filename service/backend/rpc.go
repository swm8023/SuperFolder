package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ErrorMethodNotFound = 1001
	ErrorInvalidMessage = 1002
	ErrorTimeout        = 1003
	ErrorConnectionLost = 1004
)

var EmbeddedWebFS fs.FS

type MessageKind int

const (
	MessageInvalid MessageKind = iota
	MessageCall
	MessageSuccess
	MessageFailure
	MessageNotification
)

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Message struct {
	ID      *int            `json:"id,omitempty"`
	Method  Method          `json:"method,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type ServerOptions struct {
	AppName        string
	Headless       bool
	StaticFS       fs.FS
	OnSessionReady func(ctx CallContext)
}

type Server struct {
	appName        string
	headless       bool
	staticFS       fs.FS
	onSessionReady func(ctx CallContext)
	upgrader       websocket.Upgrader
	handlers       map[Method]HandlerFunc
}

type HandlerFunc func(ctx CallContext) (any, *RPCError)
type NotifyFunc func(method Method, payload any) error
type SessionTaskFunc func(ctx context.Context, notify NotifyFunc)

type CallContext struct {
	Payload json.RawMessage
	session *wsSession
	server  *Server
}

func (c CallContext) StartSessionTask(key string, run SessionTaskFunc) bool {
	if c.session == nil {
		return false
	}
	return c.session.startTask(key, run)
}

func (c CallContext) Notify(method Method, payload any) error {
	if c.session == nil {
		return fmt.Errorf("connection lost")
	}
	return c.session.send(NotificationMessage(method, payload))
}

func DecodeMessage(data []byte) (Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func ClassifyMessage(msg Message) MessageKind {
	hasID := msg.ID != nil
	validID := hasID && *msg.ID != 0
	hasMethod := msg.Method != 0
	validMethod := msg.Method > 0
	hasPayload := msg.Payload != nil
	hasError := msg.Error != nil

	if hasID && !validID {
		return MessageInvalid
	}
	if hasMethod && !validMethod {
		return MessageInvalid
	}

	if validID && validMethod && hasPayload && !hasError {
		return MessageCall
	}

	if validID && !hasMethod && hasPayload && !hasError {
		return MessageSuccess
	}

	if validID && !hasMethod && !hasPayload && hasError {
		return MessageFailure
	}

	if !hasID && validMethod && hasPayload && !hasError {
		return MessageNotification
	}

	return MessageInvalid
}

func SuccessMessage(id int, payload any) Message {
	return Message{ID: &id, Payload: mustJSON(payload)}
}

func ErrorMessage(id int, code int, message string) Message {
	return Message{ID: &id, Error: &RPCError{Code: code, Message: message}}
}

func NotificationMessage(method Method, payload any) Message {
	return Message{Method: method, Payload: mustJSON(payload)}
}

func NewServer(options ServerOptions) *Server {
	appName := options.AppName
	if appName == "" {
		appName = "app"
	}

	staticFS := options.StaticFS
	if staticFS == nil {
		staticFS = EmbeddedWebFS
	}

	server := &Server{
		appName:        appName,
		headless:       options.Headless,
		staticFS:       staticFS,
		onSessionReady: options.OnSessionReady,
		handlers:       map[Method]HandlerFunc{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	server.registerBuiltinHandlers()
	return server
}

func (s *Server) RegisterHandler(method Method, handler HandlerFunc) {
	s.handlers[method] = handler
}

func (s *Server) registerBuiltinHandlers() {
	s.RegisterHandler(App.Hello, func(ctx CallContext) (any, *RPCError) {
		if s.onSessionReady != nil {
			s.onSessionReady(ctx)
		}
		return map[string]any{
			"app":      s.appName,
			"headless": s.headless,
		}, nil
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.URL.Path {
	case "/healthz":
		s.handleHealth(w, r)
	case "/boot":
		s.handleBoot(w, r)
	case "/ws":
		s.handleWebSocket(w, r)
	default:
		s.handleStatic(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"app": s.appName,
	})
}

func (s *Server) handleBoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}

	writeHTTPJSON(w, http.StatusOK, map[string]any{
		"app":      s.appName,
		"headless": s.headless,
		"rpcUrl":   fmt.Sprintf("%s://%s/ws", scheme, r.Host),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	session := newWSSession(s, conn)
	session.run()
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.staticFS == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}

	data, err := fs.ReadFile(s.staticFS, name)
	if err != nil {
		data, err = fs.ReadFile(s.staticFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
}

type wsSession struct {
	server  *Server
	conn    *websocket.Conn
	writeMu sync.Mutex
	tasksMu sync.Mutex
	tasks   map[string]context.CancelFunc
}

func newWSSession(server *Server, conn *websocket.Conn) *wsSession {
	return &wsSession{
		server: server,
		conn:   conn,
		tasks:  map[string]context.CancelFunc{},
	}
}

func (s *wsSession) run() {
	defer func() {
		s.cancelTasks()
		_ = s.conn.Close()
	}()

	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil {
			return
		}

		switch ClassifyMessage(msg) {
		case MessageCall:
			s.handleCall(msg)
		case MessageInvalid:
			if msg.ID != nil && *msg.ID != 0 {
				_ = s.send(ErrorMessage(*msg.ID, ErrorInvalidMessage, "invalid rpc message"))
				continue
			}
			return
		default:
			if msg.ID != nil && *msg.ID != 0 {
				_ = s.send(ErrorMessage(*msg.ID, ErrorInvalidMessage, "invalid rpc message"))
			}
		}
	}
}

func (s *wsSession) handleCall(msg Message) {
	id := *msg.ID
	handler, ok := s.server.handlers[msg.Method]
	if !ok {
		_ = s.send(ErrorMessage(id, ErrorMethodNotFound, "method not found: "+MethodName(msg.Method)))
		return
	}

	payload, rpcErr := handler(CallContext{
		Payload: msg.Payload,
		session: s,
		server:  s.server,
	})
	if rpcErr != nil {
		_ = s.send(ErrorMessage(id, rpcErr.Code, rpcErr.Message))
		return
	}

	_ = s.send(SuccessMessage(id, payload))
}

func (s *wsSession) startTask(key string, run SessionTaskFunc) bool {
	if key == "" || run == nil {
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.tasksMu.Lock()
	if _, exists := s.tasks[key]; exists {
		s.tasksMu.Unlock()
		cancel()
		return false
	}
	s.tasks[key] = cancel
	s.tasksMu.Unlock()

	go func() {
		defer func() {
			s.tasksMu.Lock()
			delete(s.tasks, key)
			s.tasksMu.Unlock()
		}()
		run(ctx, func(method Method, payload any) error {
			return s.send(NotificationMessage(method, payload))
		})
	}()

	return true
}

func (s *wsSession) cancelTasks() {
	s.tasksMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.tasks))
	for key, cancel := range s.tasks {
		cancels = append(cancels, cancel)
		delete(s.tasks, key)
	}
	s.tasksMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (s *wsSession) send(msg Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(msg)
}

func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func mustJSON(payload any) json.RawMessage {
	if payload == nil {
		return json.RawMessage(`{}`)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
