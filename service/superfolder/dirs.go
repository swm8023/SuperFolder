package superfolder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"apphostdemo/service/backend"
)

func ListChildren(req ListChildrenRequest) (ListChildrenResponse, *backend.RPCError) {
	if req.Path == "" {
		return ListChildrenResponse{}, &backend.RPCError{Code: ErrorPathNotFound, Message: "path is required"}
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ListChildrenResponse{}, &backend.RPCError{Code: ErrorPathNotFound, Message: "path not found: " + req.Path}
		}
		return ListChildrenResponse{}, toRPCError(err)
	}
	if !info.IsDir() {
		return ListChildrenResponse{}, &backend.RPCError{Code: ErrorPathNotDirectory, Message: "path is not a directory: " + req.Path}
	}

	rawEntries, err := readDirectEntries(req.Path)
	if err != nil {
		return ListChildrenResponse{}, toRPCError(err)
	}
	hash := hashEntries(rawEntries, req)
	if req.KnownHash != "" && req.KnownHash == hash {
		return ListChildrenResponse{Path: req.Path, Unchanged: true, ChildrenHash: hash}, nil
	}

	entries := filterEntries(rawEntries, req.FilterText)
	sortEntries(entries, req.SortKey, req.SortDirection)

	return ListChildrenResponse{
		Path:         req.Path,
		Unchanged:    false,
		ChildrenHash: hash,
		Entries:      entries,
	}, nil
}

func readDirectEntries(dir string) ([]DirectoryEntry, error) {
	children, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	entries := make([]DirectoryEntry, 0, len(children))
	for _, child := range children {
		info, err := child.Info()
		if err != nil {
			continue
		}
		kind := EntryKindFile
		size := info.Size()
		hasChildren := false
		if child.IsDir() {
			kind = EntryKindDirectory
			size = 0
			hasChildren = directoryHasChildren(filepath.Join(dir, child.Name()))
		}
		entries = append(entries, DirectoryEntry{
			Name:        child.Name(),
			Path:        filepath.Join(dir, child.Name()),
			Kind:        kind,
			Size:        size,
			MTime:       info.ModTime().UnixMilli(),
			Readonly:    info.Mode().Perm()&0o200 == 0,
			Hidden:      strings.HasPrefix(child.Name(), "."),
			System:      false,
			HasChildren: hasChildren,
		})
	}
	return entries, nil
}

func directoryHasChildren(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func filterEntries(entries []DirectoryEntry, filterText string) []DirectoryEntry {
	filter := strings.TrimSpace(strings.ToLower(filterText))
	if filter == "" {
		return append([]DirectoryEntry(nil), entries...)
	}
	filtered := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name), filter) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func sortEntries(entries []DirectoryEntry, key SortKey, direction SortDirection) {
	if key == "" {
		key = SortKeyName
	}
	desc := direction == SortDirectionDesc
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		cmp := 0
		switch key {
		case SortKeyKind:
			cmp = strings.Compare(string(left.Kind), string(right.Kind))
			if cmp == 0 {
				cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
			}
		case SortKeySize:
			cmp = compareInt64(left.Size, right.Size)
			if cmp == 0 {
				cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
			}
		case SortKeyMTime:
			cmp = compareInt64(left.MTime, right.MTime)
			if cmp == 0 {
				cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
			}
		default:
			cmp = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareInt64(left int64, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func hashEntries(entries []DirectoryEntry, req ListChildrenRequest) string {
	canonical := append([]DirectoryEntry(nil), entries...)
	sort.SliceStable(canonical, func(i, j int) bool {
		return strings.ToLower(canonical[i].Name) < strings.ToLower(canonical[j].Name)
	})
	data, _ := json.Marshal(struct {
		ViewMode      ViewMode         `json:"viewMode"`
		SortKey       SortKey          `json:"sortKey"`
		SortDirection SortDirection    `json:"sortDirection"`
		FilterText    string           `json:"filterText"`
		Entries       []DirectoryEntry `json:"entries"`
	}{
		ViewMode:      req.ViewMode,
		SortKey:       req.SortKey,
		SortDirection: req.SortDirection,
		FilterText:    req.FilterText,
		Entries:       canonical,
	})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func decodePayload[T any](payload json.RawMessage) (T, *backend.RPCError) {
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, invalidPayload(err)
	}
	return value, nil
}

func requirePayloadPath(path string) *backend.RPCError {
	if strings.TrimSpace(path) == "" {
		return &backend.RPCError{Code: ErrorPathNotFound, Message: "path is required"}
	}
	return nil
}

func formatPathError(prefix string, path string, err error) *backend.RPCError {
	return &backend.RPCError{Code: ErrorPathNotFound, Message: fmt.Sprintf("%s %s: %v", prefix, path, err)}
}
