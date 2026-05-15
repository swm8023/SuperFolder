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

func (a *App) ExecuteMenu(command string, selection []string, targetDir string) (any, *backend.RPCError) {
	switch command {
	case "copy":
		return map[string]any{}, a.SetClipboard(ClipboardState{Mode: ClipboardModeCopy, Paths: selection})
	case "cut":
		return map[string]any{}, a.SetClipboard(ClipboardState{Mode: ClipboardModeCut, Paths: selection})
	case "paste":
		return a.PasteClipboard(targetDir)
	case "delete":
		return a.EnqueueJob(FileJobRequest{Kind: JobKindDelete, Sources: selection, Permanent: false})
	case "delete_permanent":
		return a.EnqueueJob(FileJobRequest{Kind: JobKindDelete, Sources: selection, Permanent: true})
	default:
		return map[string]any{"handled": false}, nil
	}
}
