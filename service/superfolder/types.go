package superfolder

type ViewMode string
type SortKey string
type SortDirection string
type FavoriteKind string

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
)

type Options struct {
	ProfileDir   string
	HomeDir      string
	DesktopDir   string
	DownloadsDir string
	DocumentsDir string
}

type App struct {
	options Options
	store   *Store
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
