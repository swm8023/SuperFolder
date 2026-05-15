package superfolder

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"apphostdemo/service/backend"
)

type gitRunner func(ctx context.Context, dir string, args ...string) (string, error)

type GitService struct {
	mu       sync.Mutex
	runner   gitRunner
	cache    map[string]GitSummary
	inFlight map[string]bool
}

func NewGitService() *GitService {
	return &GitService{
		runner:   runGitCommand,
		cache:    map[string]GitSummary{},
		inFlight: map[string]bool{},
	}
}

func (g *GitService) Refresh(path string) *backend.RPCError {
	if strings.TrimSpace(path) == "" {
		return &backend.RPCError{Code: ErrorPathNotFound, Message: "path is required"}
	}
	cleanPath := filepath.Clean(path)
	g.mu.Lock()
	if g.inFlight[cleanPath] {
		g.mu.Unlock()
		return nil
	}
	g.inFlight[cleanPath] = true
	g.mu.Unlock()

	go func() {
		summary := g.loadSummary(cleanPath)
		g.mu.Lock()
		delete(g.inFlight, cleanPath)
		if summary.IsRepo {
			g.cache[summary.RepoRoot] = summary
		} else {
			g.cache[cleanPath] = summary
		}
		g.mu.Unlock()
	}()
	return nil
}

func (g *GitService) Summary(path string) GitSummary {
	cleanPath := filepath.Clean(path)
	g.mu.Lock()
	defer g.mu.Unlock()
	var best GitSummary
	bestLen := -1
	for root, summary := range g.cache {
		cleanRoot := filepath.Clean(root)
		if cleanPath == cleanRoot || strings.HasPrefix(strings.ToLower(cleanPath), strings.ToLower(cleanRoot+string(filepath.Separator))) {
			if len(cleanRoot) > bestLen {
				best = summary
				bestLen = len(cleanRoot)
			}
		}
	}
	if bestLen >= 0 {
		best.Path = path
		return best
	}
	return GitSummary{Path: path, IsRepo: false}
}

func (g *GitService) loadSummary(path string) GitSummary {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	root, err := g.runner(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(root) == "" {
		return GitSummary{Path: path, IsRepo: false}
	}
	repoRoot := filepath.Clean(strings.TrimSpace(root))
	branch, _ := g.runner(ctx, repoRoot, "branch", "--show-current")
	status, statusErr := g.runner(ctx, repoRoot, "status", "--porcelain=v1")
	logOutput, _ := g.runner(ctx, repoRoot, "log", "--oneline", "-n", "5")
	summary := GitSummary{
		Path:     path,
		IsRepo:   true,
		RepoRoot: repoRoot,
		Branch:   strings.TrimSpace(branch),
		Changed:  countNonEmptyLines(status),
		Logs:     parseGitLogs(logOutput),
	}
	if statusErr != nil {
		summary.Error = statusErr.Error()
	}
	return summary
}

func runGitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	return string(output), err
}

func countNonEmptyLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func parseGitLogs(text string) []GitLogEntry {
	logs := []GitLogEntry{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		entry := GitLogEntry{Hash: parts[0]}
		if len(parts) > 1 {
			entry.Subject = parts[1]
		}
		logs = append(logs, entry)
	}
	return logs
}
