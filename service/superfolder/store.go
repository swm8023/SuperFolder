package superfolder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu      sync.Mutex
	options Options
}

func NewStore(options Options) (*Store, error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(normalized.ProfileDir, 0o755); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}
	return &Store{options: normalized}, nil
}

func (s *Store) Session() (SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var session SessionState
	if err := readJSON(s.sessionPath(), &session); err != nil {
		if os.IsNotExist(err) {
			return defaultSession(s.options), nil
		}
		return SessionState{}, err
	}
	if len(session.Windows) == 0 {
		return defaultSession(s.options), nil
	}
	return session, nil
}

func (s *Store) SaveSession(session SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session.Version == 0 {
		session.Version = 1
	}
	return writeJSONAtomic(s.sessionPath(), session)
}

func (s *Store) Config() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var config Config
	if err := readJSON(s.configPath(), &config); err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(s.options), nil
		}
		return Config{}, err
	}
	if config.Favorites == nil {
		config.Favorites = []FavoriteItem{}
	}
	return config, nil
}

func (s *Store) SaveConfig(config Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if config.Favorites == nil {
		config.Favorites = []FavoriteItem{}
	}
	return writeJSONAtomic(s.configPath(), config)
}

func (s *Store) sessionPath() string {
	return filepath.Join(s.options.ProfileDir, "session.json")
}

func (s *Store) configPath() string {
	return filepath.Join(s.options.ProfileDir, "config.json")
}

func normalizeOptions(options Options) (Options, error) {
	if options.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Options{}, fmt.Errorf("resolve home dir: %w", err)
		}
		options.HomeDir = home
	}
	if options.DesktopDir == "" {
		options.DesktopDir = filepath.Join(options.HomeDir, "Desktop")
	}
	if options.DownloadsDir == "" {
		options.DownloadsDir = filepath.Join(options.HomeDir, "Downloads")
	}
	if options.DocumentsDir == "" {
		options.DocumentsDir = filepath.Join(options.HomeDir, "Documents")
	}
	if options.ProfileDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return Options{}, fmt.Errorf("resolve config dir: %w", err)
		}
		options.ProfileDir = filepath.Join(configDir, "SuperFolder")
	}
	return options, nil
}

func defaultConfig(options Options) Config {
	return Config{Favorites: []FavoriteItem{
		{ID: "fav-home", Name: "Home", Path: options.HomeDir, Kind: FavoriteKindFolder},
		{ID: "fav-desktop", Name: "Desktop", Path: options.DesktopDir, Kind: FavoriteKindFolder},
		{ID: "fav-downloads", Name: "Downloads", Path: options.DownloadsDir, Kind: FavoriteKindFolder},
		{ID: "fav-documents", Name: "Documents", Path: options.DocumentsDir, Kind: FavoriteKindFolder},
	}}
}

func defaultSession(options Options) SessionState {
	return SessionState{
		Version: 1,
		Windows: []WorkspaceWindowState{
			{
				ID:           "window-1",
				ActivePaneID: "pane-left",
				Panes: []PaneState{
					defaultPane("pane-left", "tab-left-home", "Home", options.HomeDir),
					defaultPane("pane-right", "tab-right-downloads", "Downloads", options.DownloadsDir),
				},
				UtilityPanel: UtilityPanelState{
					Collapsed: false,
					Height:    240,
					ActiveTab: "preview",
				},
			},
		},
	}
}

func defaultPane(paneID string, tabID string, title string, path string) PaneState {
	return PaneState{
		ID:          paneID,
		ActiveTabID: tabID,
		Tabs: []BrowserTabState{
			{
				ID:            tabID,
				Title:         title,
				Path:          path,
				ViewMode:      ViewModeDetails,
				SortKey:       SortKeyName,
				SortDirection: SortDirectionAsc,
				FilterText:    "",
				ExpandedPaths: []string{},
			},
		},
	}
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
