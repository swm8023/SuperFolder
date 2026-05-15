package superfolder

import (
	"encoding/json"
	"fmt"
	"sync"

	"apphostdemo/service/backend"
)

const ErrorSuperFolderInvalidPayload = 10000
const ErrorPathNotFound = 10001
const ErrorPathNotDirectory = 10002
const ErrorPreviewTooLarge = 10030

type App struct {
	mu        sync.Mutex
	options   Options
	store     *Store
	jobs      *JobManager
	git       *GitService
	clipboard ClipboardState
}

func NewApp(options Options) (*App, error) {
	store, err := NewStore(options)
	if err != nil {
		return nil, err
	}
	return &App{options: store.options, store: store, jobs: NewJobManager(), git: NewGitService()}, nil
}

func (a *App) GetSession() (SessionState, error) {
	return a.store.Session()
}

func (a *App) UpdateSession(session SessionState) error {
	return a.store.SaveSession(session)
}

func (a *App) GetFavorites() ([]FavoriteItem, error) {
	config, err := a.store.Config()
	if err != nil {
		return nil, err
	}
	return config.Favorites, nil
}

func (a *App) UpdateFavorites(favorites []FavoriteItem) error {
	return a.store.SaveConfig(Config{Favorites: favorites})
}

func (a *App) EnqueueJob(req FileJobRequest) (JobSnapshot, *backend.RPCError) {
	return a.jobs.Enqueue(req)
}

func (a *App) ListJobs() []JobSnapshot {
	return a.jobs.List()
}

func (a *App) CancelJob(id string) *backend.RPCError {
	return a.jobs.Cancel(id)
}

func (a *App) ResolveConflict(resolution ConflictResolution) *backend.RPCError {
	return a.jobs.ResolveConflict(resolution)
}

func (a *App) SetClipboard(clipboard ClipboardState) *backend.RPCError {
	if clipboard.Mode != ClipboardModeCopy && clipboard.Mode != ClipboardModeCut {
		return &backend.RPCError{Code: ErrorClipboardEmpty, Message: "unsupported clipboard mode"}
	}
	if len(clipboard.Paths) == 0 {
		return &backend.RPCError{Code: ErrorClipboardEmpty, Message: "clipboard requires paths"}
	}
	a.mu.Lock()
	a.clipboard = clipboard
	a.mu.Unlock()
	return nil
}

func (a *App) RefreshGitStatus(req GitStatusRefreshRequest) *backend.RPCError {
	return a.git.Refresh(req.Path)
}

func (a *App) GitSummary(path string) GitSummary {
	return a.git.Summary(path)
}

func (a *App) GetPreview(req PreviewRequest) (PreviewResponse, *backend.RPCError) {
	return GetPreview(req)
}

func (a *App) PasteClipboard(targetDir string) (JobSnapshot, *backend.RPCError) {
	a.mu.Lock()
	clipboard := a.clipboard
	a.mu.Unlock()
	if len(clipboard.Paths) == 0 {
		return JobSnapshot{}, &backend.RPCError{Code: ErrorClipboardEmpty, Message: "clipboard is empty"}
	}
	kind := JobKindCopy
	if clipboard.Mode == ClipboardModeCut {
		kind = JobKindMove
	}
	return a.EnqueueJob(FileJobRequest{Kind: kind, Sources: clipboard.Paths, TargetDir: targetDir})
}

func (a *App) Register(server *backend.Server) {
	server.RegisterHandler(backend.Folder.Session.Get, func(ctx backend.CallContext) (any, *backend.RPCError) {
		session, err := a.GetSession()
		if err != nil {
			return nil, toRPCError(err)
		}
		return map[string]any{"session": session}, nil
	})

	server.RegisterHandler(backend.Folder.Session.Update, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var payload struct {
			Session SessionState `json:"session"`
		}
		if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
			return nil, invalidPayload(err)
		}
		if err := a.UpdateSession(payload.Session); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]any{"session": payload.Session}, nil
	})

	server.RegisterHandler(backend.Folder.Favorites.List, func(ctx backend.CallContext) (any, *backend.RPCError) {
		favorites, err := a.GetFavorites()
		if err != nil {
			return nil, toRPCError(err)
		}
		return map[string]any{"favorites": favorites}, nil
	})

	server.RegisterHandler(backend.Folder.Favorites.Update, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var payload struct {
			Favorites []FavoriteItem `json:"favorites"`
		}
		if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
			return nil, invalidPayload(err)
		}
		if err := a.UpdateFavorites(payload.Favorites); err != nil {
			return nil, toRPCError(err)
		}
		return map[string]any{"favorites": payload.Favorites}, nil
	})

	server.RegisterHandler(backend.Folder.Children.List, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[ListChildrenRequest](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return ListChildren(req)
	})

	server.RegisterHandler(backend.Folder.Menu.List, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[MenuContext](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"items": a.MenuItems(req)}, nil
	})

	server.RegisterHandler(backend.Folder.Menu.Execute, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var req struct {
			Command   string   `json:"command"`
			Selection []string `json:"selection"`
			TargetDir string   `json:"targetDir"`
		}
		if err := json.Unmarshal(ctx.Payload, &req); err != nil {
			return nil, invalidPayload(err)
		}
		return a.ExecuteMenu(req.Command, req.Selection, req.TargetDir)
	})

	server.RegisterHandler(backend.Folder.Clipboard.Set, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[ClipboardState](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := a.SetClipboard(req); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"clipboard": req}, nil
	})

	server.RegisterHandler(backend.Folder.Clipboard.Paste, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var req struct {
			TargetDir string `json:"targetDir"`
		}
		if err := json.Unmarshal(ctx.Payload, &req); err != nil {
			return nil, invalidPayload(err)
		}
		job, rpcErr := a.PasteClipboard(req.TargetDir)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"job": job}, nil
	})

	server.RegisterHandler(backend.Job.List, func(ctx backend.CallContext) (any, *backend.RPCError) {
		return map[string]any{"jobs": a.ListJobs()}, nil
	})

	server.RegisterHandler(backend.Job.Cancel, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var req struct {
			JobID string `json:"jobId"`
		}
		if err := json.Unmarshal(ctx.Payload, &req); err != nil {
			return nil, invalidPayload(err)
		}
		if rpcErr := a.CancelJob(req.JobID); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"jobId": req.JobID}, nil
	})

	server.RegisterHandler(backend.Job.Conflict.Resolve, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[ConflictResolution](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := a.ResolveConflict(req); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"jobId": req.JobID}, nil
	})

	server.RegisterHandler(backend.Git.Status.Refresh, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[GitStatusRefreshRequest](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		if rpcErr := a.RefreshGitStatus(req); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"path": req.Path}, nil
	})

	server.RegisterHandler(backend.Git.Summary.Get, func(ctx backend.CallContext) (any, *backend.RPCError) {
		var req struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(ctx.Payload, &req); err != nil {
			return nil, invalidPayload(err)
		}
		return map[string]any{"summary": a.GitSummary(req.Path)}, nil
	})

	server.RegisterHandler(backend.Preview.Get, func(ctx backend.CallContext) (any, *backend.RPCError) {
		req, rpcErr := decodePayload[PreviewRequest](ctx.Payload)
		if rpcErr != nil {
			return nil, rpcErr
		}
		preview, rpcErr := a.GetPreview(req)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"preview": preview}, nil
	})
}

func invalidPayload(err error) *backend.RPCError {
	return &backend.RPCError{Code: ErrorSuperFolderInvalidPayload, Message: fmt.Sprintf("invalid payload: %v", err)}
}

func toRPCError(err error) *backend.RPCError {
	return &backend.RPCError{Code: ErrorSuperFolderInvalidPayload, Message: err.Error()}
}
