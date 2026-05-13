# SuperFolder Context

## Product Terms

### SuperFolder

SuperFolder is a backend-mediated file workspace for browsing and operating on folders, repositories, terminals, previews, and related developer/file-management tools.

### Client

A Client is any user-facing frontend that renders the SuperFolder workspace. A Client may be rendered as a Web Client or as a Native Client, but it should reach the same product capabilities through the SuperFolder RPC boundary.

### Desktop Executable

The Desktop Executable is the installed `.exe` entry point for local use. It starts the App Host/Session Backend, waits for the communication surface to become available, and then loads the frontend UI. Local use does not rely on a browser page launching the backend.

### Desktop UI Surface

The Desktop UI Surface is the frontend page loaded inside the Desktop Executable after it starts the App Host. The Windows product path uses an embedded WebView2 window. A browser or development server is only a development/debugging surface and must not define product startup semantics.

### Technology Stack

SuperFolder uses TypeScript/React for the shared frontend and Go for the Desktop Executable, App Host, Session Backend, RPC runtime, and SuperFolder backend capabilities.

### Workspace Window

A Workspace Window is an operating-system-level SuperFolder window managed by the same App Host session. Each Workspace Window owns its layout, file browser panes, tabs, bottom panel state, and session restoration data. Multiple Workspace Windows should not require multiple independent App Host instances.

### Split Pane

A Split Pane is a general layout container inside a Workspace Window. The default SuperFolder layout uses two side-by-side File Browser Panes, but the underlying layout model should support additional panes and horizontal or vertical splits over time.

### File Browser Pane

A File Browser Pane is an independently navigable file browsing area inside a Workspace Window. Each pane can own tabs, current path, view mode, sort/filter state, and visible directory data subscriptions.

### Web Client

The Web Client is the browser-rendered SuperFolder frontend. It is expected to support the full product capability set by communicating with a SuperFolder Backend, instead of relying on direct browser access to the local filesystem.

### Native Client

The Native Client is a locally installed SuperFolder frontend/runtime used for higher efficiency, stronger desktop integration, or lower-latency local operation. Native rendering is an optimization path, not a separate product capability boundary.

### SuperFolder Backend

The SuperFolder Backend owns filesystem access, repository integration, terminal sessions, preview generation, indexing, custom context-menu execution, and other privileged or heavy operations for the running SuperFolder session.

### App Host

The App Host is a project-independent local runtime that can be reused by SuperFolder and future applications. It starts with a client/app session, opens the local communication surface, serves or coordinates the frontend UI, and exposes application backend capabilities through a common RPC system.

### Session Backend

A Session Backend is the backend process for a running application session. For SuperFolder, the Session Backend runs under the App Host lifecycle and follows the app/session lifetime unless explicitly configured otherwise.

### RPC Boundary

The RPC Boundary is the product contract between Clients and the SuperFolder Backend. It should hide transport details from feature code so the same capabilities can run over local direct calls, HTTP/HTTPS, WS/WSS, or another compatible transport.

### RPC System

The RPC System is a reusable, project-independent communication layer provided by the App Host. It should support local and remote transports while presenting a transport-agnostic API to application feature code.

### Protocol Envelope

The Protocol Envelope is the stable application-level message shape used by the RPC System. It should identify the target method id, integer id, payload, protocol/app version, capability context, cancellation/deadline metadata, stream identity when relevant, and structured error information.

### RPC Message

An RPC Message is the first implementation shape for the Protocol Envelope. It uses `id`, integer `method`, `payload`, and `error` fields without a `type` discriminator. `id + method + payload` is a call, `id + payload` is a successful completion, `id + error` is a failed completion, and `method + payload` is a notification. Method ids are globally unique and never reused.

### RPC Error

An RPC Error is the structured failure payload for a failed RPC message. It uses an integer `code` plus a human-readable `message`. Codes `1-9999` are reserved for App Host/RPC built-in errors; codes `10000+` are for application capabilities.

### RPC Call Timeout

RPC calls have a default timeout of 30 seconds. Callers may override the timeout for a specific call. Timeout starts when the call is initiated and includes disconnection and reconnect waiting time. Timed-out calls fail with an App Host/RPC built-in timeout error.

### RPC Connection Loss

RPC reconnect attempts run every 200ms after a WebSocket disconnect. After 50 consecutive failed reconnect attempts, the connection is considered lost, UI/session state may reset, and pending calls fail with an App Host/RPC built-in connection-lost error. After reset, the client continues reconnecting and creates a fresh boot/session when connection succeeds.

### Headless Mode

Headless Mode starts the App Host service with an explicit port, exposes health, boot, and RPC endpoints, and does not load or open a UI surface. It is the standard mode for development automation and tests.

### Stream Payload

A Stream Payload is data delivered incrementally through the RPC System, such as terminal output, repository logs, directory change events, file preview chunks, or large binary content. The protocol may use JSON for control messages and binary chunks or dedicated streams for large payloads.

### Bidirectional RPC

Bidirectional RPC means both sides of the RPC connection can initiate calls and send push-style messages. Calls and completions are correlated by integer ids. Frontend-originated calls use positive ids; backend-originated calls use negative ids.

### View State

View State is the Client-owned, in-memory state needed to render the current interaction posture, such as scroll position, visible ranges, transient hover/focus, and other state that may reset when the app restarts. Directory expansion and data loading still go through RPC calls. View State is not business persistence.

### Persistent Configuration

Persistent Configuration is user/application configuration saved outside transient Client memory, such as favorites, custom context menu definitions, UI preferences, and other settings that should survive app restart. The Client may edit configuration, but persistence belongs behind the App Host/RPC boundary rather than ad hoc frontend storage.

### Session Persistence

Session Persistence is backend-owned storage for state that should survive app restart, such as window layout, open file/browser tabs, current paths, workspace/session restore data, view preferences, repository integration settings, and any UI state explicitly chosen for restoration.

### Transport

A Transport is the concrete communication mechanism used underneath the RPC System. Local desktop use should prefer a low-overhead IPC transport when available; remote browser access should use HTTPS/WSS. Localhost HTTPS/WSS remains useful for development, debugging, and compatibility.

## Architecture Principles

- Product capabilities should be designed backend-first and exposed through the RPC Boundary.
- Web and Native frontends should differ primarily in rendering/runtime efficiency, not in feature availability.
- Browser security limits are treated as a deployment constraint solved by a trusted Session Backend, not by reducing Web Client scope.
- The App Host and RPC System should be designed as reusable infrastructure, not as SuperFolder-specific code.
- The local desktop entry point should be a Go executable that starts the App Host before creating the embedded WebView2 UI surface.
- A single App Host session can manage multiple Workspace Windows.
- The default SuperFolder workspace layout is two side-by-side File Browser Panes, implemented on top of a general Split Pane model.
- The primary application stack is TypeScript/React frontend plus Go executable/backend. Future native window adapters must remain outside the business/RPC core.
- The RPC contract should be transport-agnostic. Transport choices are pluggable and selected by deployment/runtime context.
- The RPC framework and capability model are custom SuperFolder/App Host infrastructure, while the initial message format should remain standard, inspectable, and easy to debug.
- The protocol should support streaming, cancellation, deadlines, versioning, and structured errors from the beginning.
- The RPC System should support bidirectional calls, completions, and backend push.
- Clients may maintain transient View State in memory, but directory data loading, mutation, persistence, open file/tab state, and cross-session state belong behind the RPC Boundary.
- Persistent Configuration and Session Persistence are owned by the backend/App Host. Clients should not be the durable source of truth for these states.
