package superfolder

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apphostdemo/service/backend"
	"github.com/gorilla/websocket"
)

func TestListChildrenReturnsDirectEntriesAndHash(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "file.txt"), "hello")
	childDir := filepath.Join(root, "child")
	if err := os.Mkdir(childDir, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	mustWriteFile(t, filepath.Join(childDir, "nested.txt"), "nested")

	response, rpcErr := ListChildren(ListChildrenRequest{
		Path:          root,
		SortKey:       SortKeyName,
		SortDirection: SortDirectionAsc,
	})
	if rpcErr != nil {
		t.Fatalf("ListChildren returned error: %+v", rpcErr)
	}

	if response.Path != root || response.Unchanged {
		t.Fatalf("response metadata = %+v", response)
	}
	if response.ChildrenHash == "" {
		t.Fatalf("children hash is empty")
	}
	if len(response.Entries) != 2 {
		t.Fatalf("entries = %+v", response.Entries)
	}
	if response.Entries[0].Name != "child" || response.Entries[0].Kind != EntryKindDirectory || !response.Entries[0].HasChildren {
		t.Fatalf("first entry = %+v", response.Entries[0])
	}
	if response.Entries[1].Name != "file.txt" || response.Entries[1].Kind != EntryKindFile || response.Entries[1].HasChildren {
		t.Fatalf("second entry = %+v", response.Entries[1])
	}
}

func TestListChildrenWithKnownHashReturnsUnchanged(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "file.txt"), "hello")

	first, rpcErr := ListChildren(ListChildrenRequest{
		Path:          root,
		SortKey:       SortKeyName,
		SortDirection: SortDirectionAsc,
	})
	if rpcErr != nil {
		t.Fatalf("first ListChildren returned error: %+v", rpcErr)
	}

	second, rpcErr := ListChildren(ListChildrenRequest{
		Path:          root,
		KnownHash:     first.ChildrenHash,
		SortKey:       SortKeyName,
		SortDirection: SortDirectionAsc,
	})
	if rpcErr != nil {
		t.Fatalf("second ListChildren returned error: %+v", rpcErr)
	}
	if !second.Unchanged {
		t.Fatalf("expected unchanged response, got %+v", second)
	}
	if len(second.Entries) != 0 {
		t.Fatalf("unchanged response should not include entries: %+v", second.Entries)
	}
}

func TestListChildrenSortsAndFiltersOnBackend(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "alpha.txt"), "a")
	mustWriteFile(t, filepath.Join(root, "beta.txt"), "bbbb")
	mustWriteFile(t, filepath.Join(root, "gamma.log"), "ccc")

	response, rpcErr := ListChildren(ListChildrenRequest{
		Path:          root,
		SortKey:       SortKeySize,
		SortDirection: SortDirectionDesc,
		FilterText:    ".txt",
	})
	if rpcErr != nil {
		t.Fatalf("ListChildren returned error: %+v", rpcErr)
	}
	if got := entryNames(response.Entries); strings.Join(got, ",") != "beta.txt,alpha.txt" {
		t.Fatalf("entry order = %#v", got)
	}
}

func TestListChildrenRejectsNonDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	mustWriteFile(t, file, "hello")

	_, rpcErr := ListChildren(ListChildrenRequest{Path: file})
	if rpcErr == nil {
		t.Fatal("ListChildren accepted a file path")
	}
	if rpcErr.Code != ErrorPathNotDirectory {
		t.Fatalf("error = %+v", rpcErr)
	}
}

func TestSessionDefaultsUseHomeAndDownloads(t *testing.T) {
	app := newTestApp(t)

	session, err := app.GetSession()
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}

	if len(session.Windows) != 1 {
		t.Fatalf("window count = %d", len(session.Windows))
	}
	window := session.Windows[0]
	if len(window.Panes) != 2 {
		t.Fatalf("pane count = %d", len(window.Panes))
	}
	left := activeTab(t, window.Panes[0])
	right := activeTab(t, window.Panes[1])
	if left.Path != app.options.HomeDir {
		t.Fatalf("left path = %q, want %q", left.Path, app.options.HomeDir)
	}
	if right.Path != app.options.DownloadsDir {
		t.Fatalf("right path = %q, want %q", right.Path, app.options.DownloadsDir)
	}
	if left.ViewMode != ViewModeDetails || right.ViewMode != ViewModeDetails {
		t.Fatalf("default view modes = %q/%q", left.ViewMode, right.ViewMode)
	}
	if window.UtilityPanel.ActiveTab != "preview" || window.UtilityPanel.Height <= 0 {
		t.Fatalf("utility panel = %+v", window.UtilityPanel)
	}
}

func TestStorePersistsSessionAtomically(t *testing.T) {
	app := newTestApp(t)
	session, err := app.GetSession()
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	session.Windows[0].Panes[0].Tabs[0].Path = app.options.DocumentsDir
	session.Windows[0].Panes[0].Tabs[0].ViewMode = ViewModeTree
	session.Windows[0].Panes[0].Tabs[0].ExpandedPaths = []string{app.options.DocumentsDir}
	session.Windows[0].UtilityPanel.Collapsed = true

	if err := app.UpdateSession(session); err != nil {
		t.Fatalf("UpdateSession returned error: %v", err)
	}

	reloaded := mustNewApp(t, app.options)
	loaded, err := reloaded.GetSession()
	if err != nil {
		t.Fatalf("reloaded GetSession returned error: %v", err)
	}
	tab := activeTab(t, loaded.Windows[0].Panes[0])
	if tab.Path != app.options.DocumentsDir || tab.ViewMode != ViewModeTree {
		t.Fatalf("loaded tab = %+v", tab)
	}
	if len(tab.ExpandedPaths) != 1 || tab.ExpandedPaths[0] != app.options.DocumentsDir {
		t.Fatalf("expanded paths = %#v", tab.ExpandedPaths)
	}
	if !loaded.Windows[0].UtilityPanel.Collapsed {
		t.Fatalf("utility panel collapsed was not persisted")
	}

	if !fileExists(filepath.Join(app.options.ProfileDir, "session.json")) {
		t.Fatalf("session.json was not written")
	}
	if fileExists(filepath.Join(app.options.ProfileDir, "session.json.tmp")) {
		t.Fatalf("temporary session file was left behind")
	}
}

func TestFavoritesDefaultAndUpdateRoundTrip(t *testing.T) {
	app := newTestApp(t)

	favorites, err := app.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites returned error: %v", err)
	}
	names := favoriteNames(favorites)
	want := []string{"Home", "Desktop", "Downloads", "Documents"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("favorite names = %#v, want %#v", names, want)
	}

	next := []FavoriteItem{{ID: "fav-work", Name: "Work", Path: app.options.HomeDir, Kind: FavoriteKindFolder}}
	if err := app.UpdateFavorites(next); err != nil {
		t.Fatalf("UpdateFavorites returned error: %v", err)
	}

	reloaded := mustNewApp(t, app.options)
	loaded, err := reloaded.GetFavorites()
	if err != nil {
		t.Fatalf("reloaded GetFavorites returned error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != "Work" || loaded[0].Path != app.options.HomeDir {
		t.Fatalf("loaded favorites = %+v", loaded)
	}
}

func TestRegisterProvidesSessionAndFavoritesRPC(t *testing.T) {
	app := newTestApp(t)
	handler := backend.NewServer(backend.ServerOptions{AppName: "superfolder", Headless: true})
	app.Register(handler)
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialTestWS(t, server.URL)
	defer conn.Close()

	writeJSON(t, conn, map[string]any{"id": 1, "method": backend.Folder.Session.Get, "payload": map[string]any{}})
	sessionMsg := readByID(t, conn, 1)
	if sessionMsg.Error != nil {
		t.Fatalf("session rpc error: %+v", sessionMsg.Error)
	}
	var sessionPayload struct {
		Session SessionState `json:"session"`
	}
	if err := json.Unmarshal(sessionMsg.Payload, &sessionPayload); err != nil {
		t.Fatalf("decode session payload: %v", err)
	}
	if len(sessionPayload.Session.Windows) != 1 {
		t.Fatalf("session payload = %+v", sessionPayload.Session)
	}

	writeJSON(t, conn, map[string]any{"id": 2, "method": backend.Folder.Favorites.List, "payload": map[string]any{}})
	favoritesMsg := readByID(t, conn, 2)
	if favoritesMsg.Error != nil {
		t.Fatalf("favorites rpc error: %+v", favoritesMsg.Error)
	}
	var favoritesPayload struct {
		Favorites []FavoriteItem `json:"favorites"`
	}
	if err := json.Unmarshal(favoritesMsg.Payload, &favoritesPayload); err != nil {
		t.Fatalf("decode favorites payload: %v", err)
	}
	if len(favoritesPayload.Favorites) != 4 {
		t.Fatalf("favorites payload = %+v", favoritesPayload.Favorites)
	}
}

func TestRenameJobRenamesFileAndCompletes(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	source := filepath.Join(root, "old.txt")
	mustWriteFile(t, source, "content")

	job, rpcErr := app.EnqueueJob(FileJobRequest{
		Kind:    JobKindRename,
		Sources: []string{source},
		NewName: "new.txt",
	})
	if rpcErr != nil {
		t.Fatalf("EnqueueJob returned error: %+v", rpcErr)
	}

	completed := waitJobStatus(t, app, job.ID, JobStatusCompleted)
	if completed.Error != nil {
		t.Fatalf("job error: %+v", completed.Error)
	}
	if fileExists(source) {
		t.Fatalf("source still exists after rename")
	}
	if !fileExists(filepath.Join(root, "new.txt")) {
		t.Fatalf("renamed file does not exist")
	}
}

func TestCopyJobWaitsForConflictAndResolvesKeepBoth(t *testing.T) {
	app := newTestApp(t)
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	source := filepath.Join(sourceDir, "same.txt")
	target := filepath.Join(targetDir, "same.txt")
	mustWriteFile(t, source, "source")
	mustWriteFile(t, target, "target")

	job, rpcErr := app.EnqueueJob(FileJobRequest{
		Kind:      JobKindCopy,
		Sources:   []string{source},
		TargetDir: targetDir,
	})
	if rpcErr != nil {
		t.Fatalf("EnqueueJob returned error: %+v", rpcErr)
	}

	waiting := waitJobStatus(t, app, job.ID, JobStatusWaitingConflict)
	if waiting.Conflict == nil || waiting.Conflict.Source != source || waiting.Conflict.Target != target {
		t.Fatalf("conflict = %+v", waiting.Conflict)
	}

	if rpcErr := app.ResolveConflict(ConflictResolution{JobID: job.ID, Action: ConflictActionKeepBoth}); rpcErr != nil {
		t.Fatalf("ResolveConflict returned error: %+v", rpcErr)
	}
	completed := waitJobStatus(t, app, job.ID, JobStatusCompleted)
	if completed.Error != nil {
		t.Fatalf("job error: %+v", completed.Error)
	}
	if readFile(t, target) != "target" {
		t.Fatalf("existing target was overwritten")
	}
	if !fileExists(filepath.Join(targetDir, "same - Copy.txt")) {
		t.Fatalf("keep-both copy was not created")
	}
}

func TestMoveJobWaitsForConflictAndResolvesSkip(t *testing.T) {
	app := newTestApp(t)
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	source := filepath.Join(sourceDir, "same.txt")
	target := filepath.Join(targetDir, "same.txt")
	mustWriteFile(t, source, "source")
	mustWriteFile(t, target, "target")

	job, rpcErr := app.EnqueueJob(FileJobRequest{
		Kind:      JobKindMove,
		Sources:   []string{source},
		TargetDir: targetDir,
	})
	if rpcErr != nil {
		t.Fatalf("EnqueueJob returned error: %+v", rpcErr)
	}
	waitJobStatus(t, app, job.ID, JobStatusWaitingConflict)

	if rpcErr := app.ResolveConflict(ConflictResolution{JobID: job.ID, Action: ConflictActionSkip}); rpcErr != nil {
		t.Fatalf("ResolveConflict returned error: %+v", rpcErr)
	}
	completed := waitJobStatus(t, app, job.ID, JobStatusCompleted)
	if completed.Completed != 1 || completed.Skipped != 1 {
		t.Fatalf("completed job = %+v", completed)
	}
	if !fileExists(source) {
		t.Fatalf("source should remain after skipped move")
	}
	if readFile(t, target) != "target" {
		t.Fatalf("target changed after skipped move")
	}
}

func TestJobQueueRunsSerially(t *testing.T) {
	app := newTestApp(t)
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	conflictSource := filepath.Join(sourceDir, "same.txt")
	mustWriteFile(t, conflictSource, "source")
	mustWriteFile(t, filepath.Join(targetDir, "same.txt"), "target")
	renameSource := filepath.Join(sourceDir, "later.txt")
	mustWriteFile(t, renameSource, "later")

	first, rpcErr := app.EnqueueJob(FileJobRequest{Kind: JobKindCopy, Sources: []string{conflictSource}, TargetDir: targetDir})
	if rpcErr != nil {
		t.Fatalf("first EnqueueJob returned error: %+v", rpcErr)
	}
	second, rpcErr := app.EnqueueJob(FileJobRequest{Kind: JobKindRename, Sources: []string{renameSource}, NewName: "renamed.txt"})
	if rpcErr != nil {
		t.Fatalf("second EnqueueJob returned error: %+v", rpcErr)
	}
	waitJobStatus(t, app, first.ID, JobStatusWaitingConflict)
	if got := getJob(t, app, second.ID); got.Status != JobStatusQueued {
		t.Fatalf("second job status while first waits = %+v", got)
	}

	if rpcErr := app.ResolveConflict(ConflictResolution{JobID: first.ID, Action: ConflictActionSkip}); rpcErr != nil {
		t.Fatalf("ResolveConflict returned error: %+v", rpcErr)
	}
	waitJobStatus(t, app, first.ID, JobStatusCompleted)
	waitJobStatus(t, app, second.ID, JobStatusCompleted)
	if !fileExists(filepath.Join(sourceDir, "renamed.txt")) {
		t.Fatalf("second job did not run after first completed")
	}
}

func TestClipboardPasteCreatesCopyOrMoveJob(t *testing.T) {
	app := newTestApp(t)
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	source := filepath.Join(sourceDir, "copy.txt")
	mustWriteFile(t, source, "copy")

	if rpcErr := app.SetClipboard(ClipboardState{Mode: ClipboardModeCopy, Paths: []string{source}, SourcePaneID: "pane-left", SourceTabID: "tab-left"}); rpcErr != nil {
		t.Fatalf("SetClipboard returned error: %+v", rpcErr)
	}
	job, rpcErr := app.PasteClipboard(targetDir)
	if rpcErr != nil {
		t.Fatalf("PasteClipboard returned error: %+v", rpcErr)
	}
	waitJobStatus(t, app, job.ID, JobStatusCompleted)
	if !fileExists(filepath.Join(targetDir, "copy.txt")) {
		t.Fatalf("clipboard copy target missing")
	}
}

func TestMenuListReturnsBuiltInItems(t *testing.T) {
	app := newTestApp(t)
	items := app.MenuItems(MenuContext{Selection: []string{"C:\\tmp\\file.txt"}, CanPaste: true})
	ids := menuIDs(items)
	want := []string{"open", "open_new_tab", "copy", "cut", "paste", "rename", "delete", "delete_permanent", "properties", "copy_path"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("menu ids = %#v, want %#v", ids, want)
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	return mustNewApp(t, Options{
		ProfileDir:   t.TempDir(),
		HomeDir:      filepath.Join(t.TempDir(), "Home"),
		DesktopDir:   filepath.Join(t.TempDir(), "Desktop"),
		DownloadsDir: filepath.Join(t.TempDir(), "Downloads"),
		DocumentsDir: filepath.Join(t.TempDir(), "Documents"),
	})
}

func mustNewApp(t *testing.T, options Options) *App {
	t.Helper()
	app, err := NewApp(options)
	if err != nil {
		t.Fatalf("NewApp returned error: %v", err)
	}
	return app
}

func activeTab(t *testing.T, pane PaneState) BrowserTabState {
	t.Helper()
	for _, tab := range pane.Tabs {
		if tab.ID == pane.ActiveTabID {
			return tab
		}
	}
	t.Fatalf("active tab %q not found in %+v", pane.ActiveTabID, pane.Tabs)
	return BrowserTabState{}
}

func favoriteNames(items []FavoriteItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func entryNames(entries []DirectoryEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func waitJobStatus(t *testing.T, app *App, id string, status JobStatus) JobSnapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job := getJob(t, app, id)
		if job.Status == status {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach status %s; last=%+v", id, status, getJob(t, app, id))
	return JobSnapshot{}
}

func getJob(t *testing.T, app *App, id string) JobSnapshot {
	t.Helper()
	for _, job := range app.ListJobs() {
		if job.ID == id {
			return job
		}
	}
	t.Fatalf("job %s not found in %+v", id, app.ListJobs())
	return JobSnapshot{}
}

func menuIDs(items []MenuItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dialTestWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
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
