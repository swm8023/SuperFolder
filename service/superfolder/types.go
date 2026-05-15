package superfolder

type ViewMode string
type SortKey string
type SortDirection string
type FavoriteKind string
type EntryKind string
type JobKind string
type JobStatus string
type ConflictAction string
type ClipboardMode string
type PreviewKind string

const (
	ViewModeDetails ViewMode = "details"
	ViewModeTiles   ViewMode = "tiles"
	ViewModeTree    ViewMode = "tree"

	SortKeyName  SortKey = "name"
	SortKeyKind  SortKey = "kind"
	SortKeySize  SortKey = "size"
	SortKeyMTime SortKey = "mtime"

	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"

	FavoriteKindFolder FavoriteKind = "folder"

	EntryKindFile      EntryKind = "file"
	EntryKindDirectory EntryKind = "directory"

	JobKindCopy   JobKind = "copy"
	JobKindMove   JobKind = "move"
	JobKindDelete JobKind = "delete"
	JobKindRename JobKind = "rename"

	JobStatusQueued          JobStatus = "queued"
	JobStatusRunning         JobStatus = "running"
	JobStatusWaitingConflict JobStatus = "waiting_conflict"
	JobStatusCancelling      JobStatus = "cancelling"
	JobStatusCompleted       JobStatus = "completed"
	JobStatusFailed          JobStatus = "failed"
	JobStatusCancelled       JobStatus = "cancelled"

	ConflictActionOverwrite ConflictAction = "overwrite"
	ConflictActionSkip      ConflictAction = "skip"
	ConflictActionKeepBoth  ConflictAction = "keep_both"

	ClipboardModeCopy ClipboardMode = "copy"
	ClipboardModeCut  ClipboardMode = "cut"

	PreviewKindText  PreviewKind = "text"
	PreviewKindImage PreviewKind = "image"
)

type Options struct {
	ProfileDir   string
	HomeDir      string
	DesktopDir   string
	DownloadsDir string
	DocumentsDir string
}

type Config struct {
	Favorites []FavoriteItem `json:"favorites"`
}

type FavoriteItem struct {
	ID   string       `json:"id"`
	Name string       `json:"name"`
	Path string       `json:"path"`
	Kind FavoriteKind `json:"kind"`
}

type SessionState struct {
	Version int                    `json:"version"`
	Windows []WorkspaceWindowState `json:"windows"`
}

type WorkspaceWindowState struct {
	ID           string            `json:"id"`
	Panes        []PaneState       `json:"panes"`
	ActivePaneID string            `json:"activePaneId"`
	UtilityPanel UtilityPanelState `json:"utilityPanel"`
}

type PaneState struct {
	ID          string            `json:"id"`
	Tabs        []BrowserTabState `json:"tabs"`
	ActiveTabID string            `json:"activeTabId"`
}

type BrowserTabState struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Path          string        `json:"path"`
	ViewMode      ViewMode      `json:"viewMode"`
	SortKey       SortKey       `json:"sortKey"`
	SortDirection SortDirection `json:"sortDirection"`
	FilterText    string        `json:"filterText"`
	ExpandedPaths []string      `json:"expandedPaths"`
}

type UtilityPanelState struct {
	Collapsed bool   `json:"collapsed"`
	Height    int    `json:"height"`
	ActiveTab string `json:"activeTab"`
}

type ListChildrenRequest struct {
	Path          string        `json:"path"`
	KnownHash     string        `json:"knownHash"`
	ViewMode      ViewMode      `json:"viewMode"`
	SortKey       SortKey       `json:"sortKey"`
	SortDirection SortDirection `json:"sortDirection"`
	FilterText    string        `json:"filterText"`
}

type ListChildrenResponse struct {
	Path         string           `json:"path"`
	Unchanged    bool             `json:"unchanged"`
	ChildrenHash string           `json:"childrenHash"`
	Entries      []DirectoryEntry `json:"entries,omitempty"`
}

type DirectoryEntry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Kind        EntryKind `json:"kind"`
	Size        int64     `json:"size"`
	MTime       int64     `json:"mtime"`
	Readonly    bool      `json:"readonly"`
	Hidden      bool      `json:"hidden"`
	System      bool      `json:"system"`
	HasChildren bool      `json:"hasChildren"`
}

type FileJobRequest struct {
	Kind      JobKind  `json:"kind"`
	Sources   []string `json:"sources"`
	TargetDir string   `json:"targetDir"`
	NewName   string   `json:"newName"`
	Permanent bool     `json:"permanent"`
}

type JobSnapshot struct {
	ID        string         `json:"id"`
	Kind      JobKind        `json:"kind"`
	Status    JobStatus      `json:"status"`
	Sources   []string       `json:"sources"`
	TargetDir string         `json:"targetDir"`
	NewName   string         `json:"newName"`
	Total     int            `json:"total"`
	Completed int            `json:"completed"`
	Skipped   int            `json:"skipped"`
	Error     *backendError  `json:"error,omitempty"`
	Conflict  *ConflictState `json:"conflict,omitempty"`
}

type backendError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ConflictState struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type ConflictResolution struct {
	JobID      string         `json:"jobId"`
	Action     ConflictAction `json:"action"`
	ApplyToAll bool           `json:"applyToAll"`
}

type ClipboardState struct {
	Mode         ClipboardMode `json:"mode"`
	Paths        []string      `json:"paths"`
	SourcePaneID string        `json:"sourcePaneId"`
	SourceTabID  string        `json:"sourceTabId"`
}

type MenuContext struct {
	Selection []string `json:"selection"`
	CanPaste  bool     `json:"canPaste"`
}

type MenuItem struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type GitStatusRefreshRequest struct {
	Path string `json:"path"`
}

type GitSummary struct {
	Path     string        `json:"path"`
	IsRepo   bool          `json:"isRepo"`
	RepoRoot string        `json:"repoRoot"`
	Branch   string        `json:"branch"`
	Changed  int           `json:"changed"`
	Logs     []GitLogEntry `json:"logs"`
	Error    string        `json:"error,omitempty"`
}

type GitLogEntry struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
}

type PreviewRequest struct {
	Path string `json:"path"`
}

type PreviewResponse struct {
	Path      string      `json:"path"`
	Kind      PreviewKind `json:"kind"`
	Mime      string      `json:"mime"`
	Text      string      `json:"text,omitempty"`
	DataURL   string      `json:"dataUrl,omitempty"`
	Truncated bool        `json:"truncated"`
}
