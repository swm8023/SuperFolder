package superfolder

import (
	"encoding/json"
	"fmt"

	"apphostdemo/service/backend"
)

const ErrorSuperFolderInvalidPayload = 10000
const ErrorPathNotFound = 10001
const ErrorPathNotDirectory = 10002

func NewApp(options Options) (*App, error) {
	store, err := NewStore(options)
	if err != nil {
		return nil, err
	}
	return &App{options: store.options, store: store}, nil
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
}

func invalidPayload(err error) *backend.RPCError {
	return &backend.RPCError{Code: ErrorSuperFolderInvalidPayload, Message: fmt.Sprintf("invalid payload: %v", err)}
}

func toRPCError(err error) *backend.RPCError {
	return &backend.RPCError{Code: ErrorSuperFolderInvalidPayload, Message: err.Error()}
}
