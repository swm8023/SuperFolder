package superfolder

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"apphostdemo/service/backend"
)

const textPreviewLimit = 256 * 1024
const imagePreviewLimit = 2 * 1024 * 1024

func GetPreview(req PreviewRequest) (PreviewResponse, *backend.RPCError) {
	if strings.TrimSpace(req.Path) == "" {
		return PreviewResponse{}, &backend.RPCError{Code: ErrorPathNotFound, Message: "path is required"}
	}
	info, err := os.Stat(req.Path)
	if err != nil {
		return PreviewResponse{}, toRPCError(err)
	}
	if info.IsDir() {
		return PreviewResponse{}, &backend.RPCError{Code: ErrorPathNotDirectory, Message: "preview path is a directory: " + req.Path}
	}
	if mime := imageMime(req.Path); mime != "" {
		if info.Size() > imagePreviewLimit {
			return PreviewResponse{}, &backend.RPCError{Code: ErrorPreviewTooLarge, Message: "image is too large to preview"}
		}
		data, err := os.ReadFile(req.Path)
		if err != nil {
			return PreviewResponse{}, toRPCError(err)
		}
		return PreviewResponse{
			Path:    req.Path,
			Kind:    PreviewKindImage,
			Mime:    mime,
			DataURL: fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)),
		}, nil
	}

	if isTextPreview(req.Path) || info.Size() <= textPreviewLimit {
		file, err := os.Open(req.Path)
		if err != nil {
			return PreviewResponse{}, toRPCError(err)
		}
		defer file.Close()
		limit := textPreviewLimit
		data := make([]byte, limit)
		n, err := file.Read(data)
		if err != nil && n == 0 {
			return PreviewResponse{}, toRPCError(err)
		}
		return PreviewResponse{
			Path:      req.Path,
			Kind:      PreviewKindText,
			Mime:      "text/plain",
			Text:      string(data[:n]),
			Truncated: info.Size() > int64(limit),
		}, nil
	}

	return PreviewResponse{}, &backend.RPCError{Code: ErrorPreviewTooLarge, Message: "file is too large to preview"}
}

func imageMime(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func isTextPreview(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md", ".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".css", ".html", ".xml", ".log", ".yaml", ".yml":
		return true
	default:
		return false
	}
}
