package superfolder

import "apphostdemo/service/backend"

func (a *App) MenuItems(context MenuContext) []MenuItem {
	hasSelection := len(context.Selection) > 0
	return []MenuItem{
		{ID: "open", Label: "Open", Enabled: hasSelection},
		{ID: "open_new_tab", Label: "Open in New Tab", Enabled: hasSelection},
		{ID: "copy", Label: "Copy", Enabled: hasSelection},
		{ID: "cut", Label: "Cut", Enabled: hasSelection},
		{ID: "paste", Label: "Paste", Enabled: context.CanPaste},
		{ID: "rename", Label: "Rename", Enabled: len(context.Selection) == 1},
		{ID: "delete", Label: "Delete", Enabled: hasSelection},
		{ID: "delete_permanent", Label: "Delete Permanently", Enabled: hasSelection},
		{ID: "properties", Label: "Properties", Enabled: hasSelection},
		{ID: "copy_path", Label: "Copy Path", Enabled: hasSelection},
	}
}

func (a *App) OpenPath(path string) *backend.RPCError {
	if path == "" {
		return &backend.RPCError{Code: ErrorPathNotFound, Message: "path is required"}
	}
	if err := openPathWithShell(path); err != nil {
		return &backend.RPCError{Code: ErrorFileOperationFailed, Message: err.Error()}
	}
	return nil
}

func (a *App) ExecuteMenu(command string, selection []string, targetDir string, newName string) (any, *backend.RPCError) {
	switch command {
	case "open":
		if len(selection) == 0 {
			return nil, &backend.RPCError{Code: ErrorPathNotFound, Message: "open requires selection"}
		}
		if rpcErr := a.OpenPath(selection[0]); rpcErr != nil {
			return nil, rpcErr
		}
		return map[string]any{"opened": selection[0]}, nil
	case "copy":
		return map[string]any{}, a.SetClipboard(ClipboardState{Mode: ClipboardModeCopy, Paths: selection})
	case "cut":
		return map[string]any{}, a.SetClipboard(ClipboardState{Mode: ClipboardModeCut, Paths: selection})
	case "paste":
		return a.PasteClipboard(targetDir)
	case "rename":
		return a.EnqueueJob(FileJobRequest{Kind: JobKindRename, Sources: selection, NewName: newName})
	case "delete":
		return a.EnqueueJob(FileJobRequest{Kind: JobKindDelete, Sources: selection, Permanent: false})
	case "delete_permanent":
		return a.EnqueueJob(FileJobRequest{Kind: JobKindDelete, Sources: selection, Permanent: true})
	default:
		return map[string]any{"handled": false}, nil
	}
}
