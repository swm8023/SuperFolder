# SuperFolder MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable SuperFolder file manager prototype on top of the existing Go App Host/RPC foundation.

**Architecture:** Keep `service/backend` as reusable App Host/RPC infrastructure and add `service/superfolder` for product capabilities. Replace the demo UI with a React file workspace that talks only through generated integer RPC methods. Implement the MVP as vertical slices with tests around protocol, filesystem behavior, persistence, jobs, shortcuts, and reducers.

**Tech Stack:** TypeScript, React, Vite, Vitest, Go 1.26, gorilla/websocket, Windows WebView2, Windows batch scripts.

---

## File Structure

Create:

- `service/superfolder/types.go`: shared request/response/session/job/menu/Git/preview types.
- `service/superfolder/store.go`: Windows profile directory resolution, config/session load/save, atomic JSON writes.
- `service/superfolder/dirs.go`: direct-child directory loading, sorting/filtering, file attributes, children hash.
- `service/superfolder/jobs.go`: serial job queue, copy/move/delete/rename jobs, conflict state, cancellation.
- `service/superfolder/git.go`: async Git refresh, repo detection, status summary, status cache.
- `service/superfolder/preview.go`: text/image preview with size limits.
- `service/superfolder/menu.go`: built-in context menu registry and execution dispatch.
- `service/superfolder/app.go`: app state, RPC registration, session notification task.
- `service/superfolder/superfolder_test.go`: merged backend business tests.
- `app/src/superfolder/types.ts`: frontend product types.
- `app/src/superfolder/api.ts`: typed RPC wrapper functions.
- `app/src/superfolder/state.ts`: workspace reducer, tree cache, selection helpers.
- `app/src/superfolder/shortcuts.ts`: keyboard command mapping.
- `app/src/superfolder/components.tsx`: focused UI components for panes, tabs, lists, utility panel, dialogs.
- `app/src/tests/superfolder.test.ts`: frontend reducer/shortcut/API tests.

Modify:

- `app/src/rpc/methods.json`: replace `demo.*` with product methods.
- `app/src/App.tsx`: replace demo page with SuperFolder workspace.
- `app/src/styles.css`: replace demo styling with file manager UI styling.
- `app/package.json`: rename package to `superfolder`.
- `service/main.go`: set product identity and register `service/superfolder`.
- `service/main_test.go`: update app handler tests from demo methods to product methods.
- `service/backend/backend_test.go`: replace demo tick assumptions with generic app handshake tests.
- `script/build.bat`, `script/test.bat`, `script/smoke-headless.mjs`, `bin/start-headless.bat`, `README.md`, docs as needed: output `superfolder.exe`.

---

### Task 1: Product Identity and RPC Method Registry

**Files:**
- Modify: `app/src/rpc/methods.json`
- Modify: `service/main.go`
- Modify: `app/package.json`
- Modify: `script/build.bat`
- Modify: `script/test.bat`
- Modify: `script/smoke-headless.mjs`
- Modify: `bin/start-headless.bat`
- Modify: `README.md`
- Modify: `service/main_test.go`

- [ ] **Step 1: Write failing identity test**

Add or update a Go test that creates the app handler and calls `app.hello`. Expected payload:

```go
if helloPayload.App != "superfolder" || !helloPayload.Headless {
  t.Fatalf("hello payload = %+v", helloPayload)
}
```

Run:

```bat
cd service
go test ./...
```

Expected: FAIL because app name is still `app-host-demo`.

- [ ] **Step 2: Replace product identity**

Change:

```go
const appName = "superfolder"
```

and `WindowTitle: "SuperFolder"`. Rename package metadata in `app/package.json` to `superfolder`.

- [ ] **Step 3: Replace demo RPC methods**

Set `app/src/rpc/methods.json` methods to:

```json
{
  "app.hello": 1000001,
  "folder.session.get": 2000001,
  "folder.session.update": 2000002,
  "folder.favorites.list": 2000003,
  "folder.favorites.update": 2000004,
  "folder.children.list": 2000005,
  "folder.open": 2000006,
  "folder.menu.list": 2000007,
  "folder.menu.execute": 2000008,
  "folder.clipboard.set": 2000009,
  "folder.clipboard.paste": 2000010,
  "folder.children.updated": 2000011,
  "job.list": 2000012,
  "job.cancel": 2000013,
  "job.resolveConflict": 2000014,
  "job.updated": 2000015,
  "git.status.refresh": 2000016,
  "git.summary.get": 2000017,
  "git.status.updated": 2000018,
  "preview.get": 2000019,
  "preview.updated": 2000020
}
```

Update `ranges.default.next` to `2000021`, run:

```bat
script\codegen-methods.bat
```

Expected: generated Go and TS method constants expose `backend.Folder`, `backend.Job`, `backend.Git`, and `backend.Preview`.

- [ ] **Step 4: Rename build output**

Update scripts to output and smoke-test `bin\superfolder.exe`. `bin\start-headless.bat` should look for `superfolder.exe`.

- [ ] **Step 5: Verify identity**

Run:

```bat
script\test.bat
```

Expected: tests pass after later tasks provide missing product handlers; before handlers exist, method-not-found failures are expected and should drive Task 2.

---

### Task 2: SuperFolder App State, Session, and Favorites

**Files:**
- Create: `service/superfolder/types.go`
- Create: `service/superfolder/store.go`
- Create: `service/superfolder/app.go`
- Create: `service/superfolder/superfolder_test.go`
- Modify: `service/main.go`

- [ ] **Step 1: Write failing persistence tests**

In `service/superfolder/superfolder_test.go`, add tests for:

```go
func TestSessionDefaultsUseHomeAndDownloads(t *testing.T)
func TestStorePersistsSessionAtomically(t *testing.T)
func TestFavoritesDefaultAndUpdateRoundTrip(t *testing.T)
```

Use `t.TempDir()` as profile dir, explicit home/downloads paths, and assert defaults include two panes and four favorites.

Run:

```bat
cd service
go test ./superfolder
```

Expected: FAIL because package does not exist.

- [ ] **Step 2: Implement types and store**

Define `SessionState`, `WorkspaceWindowState`, `PaneState`, `BrowserTabState`, `UtilityPanelState`, `FavoriteItem`, `Config`, and `Store`. Implement atomic JSON save with temp file and rename.

- [ ] **Step 3: Register session and favorites RPC handlers**

`superfolder.Register(server, options)` registers:

- `folder.session.get`
- `folder.session.update`
- `folder.favorites.list`
- `folder.favorites.update`

The handlers decode `ctx.Payload`, call store methods, and return payloads with `payload` fields only.

- [ ] **Step 4: Wire main.go**

`main.go` creates the SuperFolder app state once per process and registers it with the backend server inside `NewHandler`.

- [ ] **Step 5: Verify**

Run:

```bat
cd service
go test ./superfolder ./...
```

Expected: persistence tests pass.

---

### Task 3: Directory Lazy Loading and Hash Protocol

**Files:**
- Create: `service/superfolder/dirs.go`
- Modify: `service/superfolder/app.go`
- Modify: `service/superfolder/superfolder_test.go`

- [ ] **Step 1: Write failing directory tests**

Add tests:

```go
func TestListChildrenReturnsDirectEntriesAndHash(t *testing.T)
func TestListChildrenWithKnownHashReturnsUnchanged(t *testing.T)
func TestListChildrenSortsAndFiltersOnBackend(t *testing.T)
func TestListChildrenRejectsNonDirectory(t *testing.T)
```

Use temp directories and files with controlled names. Assert direct children only, `hasChildren` for directories, sorting by name/size/mtime, and filter text.

Run:

```bat
cd service
go test ./superfolder
```

Expected: FAIL because directory loading is missing.

- [ ] **Step 2: Implement direct-child loader**

Implement:

```go
func ListChildren(req ListChildrenRequest) (ListChildrenResponse, *backend.RPCError)
```

Hash input includes name, kind, size, mtime, readonly, hidden, system, and hasChildren. Git/Preview data is not included.

- [ ] **Step 3: Register `folder.children.list`**

Decode request, call `ListChildren`, return response.

- [ ] **Step 4: Verify**

Run:

```bat
cd service
go test ./superfolder
```

Expected: directory tests pass.

---

### Task 4: Job Queue, File Operations, Clipboard, and Menu Registry

**Files:**
- Create: `service/superfolder/jobs.go`
- Create: `service/superfolder/menu.go`
- Modify: `service/superfolder/app.go`
- Modify: `service/superfolder/superfolder_test.go`

- [ ] **Step 1: Write failing job tests**

Add tests:

```go
func TestRenameJobRenamesFileAndCompletes(t *testing.T)
func TestCopyJobWaitsForConflictAndResolvesKeepBoth(t *testing.T)
func TestMoveJobWaitsForConflictAndResolvesSkip(t *testing.T)
func TestJobQueueRunsSerially(t *testing.T)
func TestClipboardPasteCreatesCopyOrMoveJob(t *testing.T)
func TestMenuListReturnsBuiltInItems(t *testing.T)
```

Use temp directories. Avoid destructive recycle-bin delete in tests; cover permanent delete with temp files and assert ordinary delete routes through the delete mode flag.

Run:

```bat
cd service
go test ./superfolder
```

Expected: FAIL because jobs/menu are missing.

- [ ] **Step 2: Implement serial job manager**

Implement a single worker queue with snapshots and `job.updated` events. Jobs support queued/running/waiting_conflict/completed/failed/cancelled.

- [ ] **Step 3: Implement copy/move/rename/delete**

Copy and move handle conflicts with overwrite/skip/keep_both plus apply-to-all. Rename fails on conflicts. Permanent delete uses `os.Remove`/`os.RemoveAll`; recycle-bin delete uses Windows shell API in production path.

- [ ] **Step 4: Implement app clipboard and menu registry**

Register built-in menu items and map menu commands to open, copy, cut, paste, rename, delete, permanent delete, properties, and copy path.

- [ ] **Step 5: Register job and menu RPC handlers**

Register:

- `folder.menu.list`
- `folder.menu.execute`
- `folder.clipboard.set`
- `folder.clipboard.paste`
- `job.list`
- `job.cancel`
- `job.resolveConflict`

- [ ] **Step 6: Verify**

Run:

```bat
cd service
go test ./superfolder
```

Expected: job and menu tests pass.

---

### Task 5: Git and Preview Capabilities

**Files:**
- Create: `service/superfolder/git.go`
- Create: `service/superfolder/preview.go`
- Modify: `service/superfolder/app.go`
- Modify: `service/superfolder/superfolder_test.go`

- [ ] **Step 1: Write failing Git/Preview tests**

Add tests:

```go
func TestGitRefreshReturnsQuicklyAndUpdatesCacheAsync(t *testing.T)
func TestGitSummaryHandlesNonRepo(t *testing.T)
func TestPreviewReadsFirst256KBOfText(t *testing.T)
func TestPreviewRejectsLargeUnknownFile(t *testing.T)
func TestPreviewClassifiesImageByExtension(t *testing.T)
```

Use dependency injection for command execution so tests do not require a real Git repo.

- [ ] **Step 2: Implement async Git service**

`git.status.refresh` schedules refresh and returns immediately. Refresh updates cache and emits `git.status.updated`. `git.summary.get` returns current cache or non-repo summary.

- [ ] **Step 3: Implement preview service**

Text reads up to 256KB. Image returns a safe data reference payload for supported extensions. Oversized or unsupported files return structured error.

- [ ] **Step 4: Verify**

Run:

```bat
cd service
go test ./superfolder
```

Expected: Git and Preview tests pass.

---

### Task 6: Frontend Product Types, API, Reducer, and Shortcuts

**Files:**
- Create: `app/src/superfolder/types.ts`
- Create: `app/src/superfolder/api.ts`
- Create: `app/src/superfolder/state.ts`
- Create: `app/src/superfolder/shortcuts.ts`
- Create: `app/src/tests/superfolder.test.ts`

- [ ] **Step 1: Write failing frontend tests**

Tests cover:

```ts
test('stores changed children by path and keeps existing children on unchanged response')
test('updates active tab path and view mode through reducer actions')
test('maps explorer keyboard shortcuts to commands')
test('api wrapper calls generated folder methods')
```

Run:

```bat
cd app
npm test -- src/tests/superfolder.test.ts
```

Expected: FAIL because modules do not exist.

- [ ] **Step 2: Implement types and typed API wrappers**

Wrap `RpcClient.call` with product functions such as `getSession`, `listChildren`, `setClipboard`, `pasteClipboard`, `cancelJob`, `refreshGitStatus`, and `getPreview`.

- [ ] **Step 3: Implement reducer and shortcut mapping**

Reducer owns frontend view state only: tree cache, selections, pending inline rename, active focus, conflict dialog. Persistent state is sent back to backend through API calls.

- [ ] **Step 4: Verify**

Run:

```bat
cd app
npm test -- src/tests/superfolder.test.ts
```

Expected: frontend product tests pass.

---

### Task 7: React SuperFolder Workspace UI

**Files:**
- Modify: `app/src/App.tsx`
- Create: `app/src/superfolder/components.tsx`
- Modify: `app/src/styles.css`
- Modify: `app/src/tests/rpc.test.ts`

- [ ] **Step 1: Replace demo shell**

`App.tsx` should create one `RpcClient`, wait for `app.hello`, then load `folder.session.get`, `folder.favorites.list`, initial pane directories, job list, and Git/Preview initial state.

- [ ] **Step 2: Build workspace UI components**

Implement Sidebar favorites, BrowserPane, TabStrip, PathBar, DetailsView, TilesView, TreeView, UtilityPanel, JobQueue, ContextMenu, ConflictDialog, and PreviewPanel.

- [ ] **Step 3: Wire interactions**

Implement navigation, tab create/close, view mode switch, sorting/filtering, tree expansion, selection, inline rename, right-click menu, keyboard shortcuts, clipboard commands, job conflict resolution, Git refresh, and Preview selection.

- [ ] **Step 4: Verify frontend**

Run:

```bat
cd app
npm run typecheck
npm test
```

Expected: typecheck and Vitest pass.

---

### Task 8: End-to-End Build, Smoke, and Docs

**Files:**
- Modify: `script/build.bat`
- Modify: `script/test.bat`
- Modify: `script/smoke-headless.mjs`
- Modify: `bin/start-headless.bat`
- Modify: `README.md`
- Modify: `docs/app-host-demo-design.md` only if stale demo instructions conflict with product usage.

- [ ] **Step 1: Update scripts**

All scripts should use `superfolder.exe`. `script\test.bat` should smoke-test `/boot` with `app == "superfolder"` and `rpcUrl` ending with `/ws`.

- [ ] **Step 2: Run full verification**

Run:

```bat
script\test.bat
```

Expected:

- Go tests pass.
- TypeScript typecheck passes.
- Vitest tests pass.
- `bin\superfolder.exe` exists.
- headless smoke test passes.

- [ ] **Step 3: Manual headless verification**

Run:

```bat
bin\start-headless.bat 18082
```

Open the printed local URL in Edge, verify the workspace loads, browse a local directory, and stop the process after verification.

- [ ] **Step 4: Final status**

Run:

```bat
git status --short
```

Expected: only intentional source changes are present; ignored `.build/`, `.superpowers/`, `node_modules/`, and exe files stay ignored.

---

## Self-Review

- Spec coverage: covers product identity, backend/frontend boundary, RPC methods, layout, session/favorites, directory hash protocol, file operations, job queue, conflicts, shortcuts, menu, Git, Preview, testing, build, and smoke verification.
- Placeholder scan: no unresolved placeholders or unspecified buckets.
- Type consistency: Go/TypeScript method names use the generated `rpc.folder.*`, `backend.Folder.*`, `rpc.job.*`, `backend.Job.*`, `rpc.git.*`, `backend.Git.*`, and `rpc.preview.*`, `backend.Preview.*` naming style.
