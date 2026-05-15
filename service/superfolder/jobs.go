package superfolder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"apphostdemo/service/backend"
)

const ErrorJobNotFound = 10020
const ErrorJobConflictRequired = 10021
const ErrorFileOperationFailed = 10022
const ErrorClipboardEmpty = 10023

type JobManager struct {
	mu    sync.Mutex
	next  int
	jobs  map[string]*fileJob
	order []string
	queue chan *fileJob
}

type fileJob struct {
	req        FileJobRequest
	snapshot   JobSnapshot
	cancel     context.CancelFunc
	conflictCh chan ConflictResolution
	applyAll   *ConflictResolution
}

func NewJobManager() *JobManager {
	manager := &JobManager{
		jobs:  map[string]*fileJob{},
		queue: make(chan *fileJob, 128),
	}
	go manager.run()
	return manager
}

func (m *JobManager) Enqueue(req FileJobRequest) (JobSnapshot, *backend.RPCError) {
	if len(req.Sources) == 0 {
		return JobSnapshot{}, &backend.RPCError{Code: ErrorFileOperationFailed, Message: "job requires at least one source"}
	}
	if req.Kind == JobKindCopy || req.Kind == JobKindMove {
		if strings.TrimSpace(req.TargetDir) == "" {
			return JobSnapshot{}, &backend.RPCError{Code: ErrorFileOperationFailed, Message: "copy/move job requires targetDir"}
		}
	}
	if req.Kind == JobKindRename && strings.TrimSpace(req.NewName) == "" {
		return JobSnapshot{}, &backend.RPCError{Code: ErrorFileOperationFailed, Message: "rename job requires newName"}
	}

	m.mu.Lock()
	m.next++
	id := fmt.Sprintf("job-%d", m.next)
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	job := &fileJob{
		req:        req,
		cancel:     cancel,
		conflictCh: make(chan ConflictResolution, 1),
		snapshot: JobSnapshot{
			ID:        id,
			Kind:      req.Kind,
			Status:    JobStatusQueued,
			Sources:   append([]string(nil), req.Sources...),
			TargetDir: req.TargetDir,
			NewName:   req.NewName,
			Total:     len(req.Sources),
		},
	}
	m.jobs[id] = job
	m.order = append(m.order, id)
	m.mu.Unlock()

	m.queue <- job
	return job.snapshot, nil
}

func (m *JobManager) List() []JobSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshots := make([]JobSnapshot, 0, len(m.order))
	for _, id := range m.order {
		snapshots = append(snapshots, cloneJobSnapshot(m.jobs[id].snapshot))
	}
	return snapshots
}

func (m *JobManager) Cancel(id string) *backend.RPCError {
	m.mu.Lock()
	job := m.jobs[id]
	if job == nil {
		m.mu.Unlock()
		return &backend.RPCError{Code: ErrorJobNotFound, Message: "job not found: " + id}
	}
	if job.snapshot.Status == JobStatusQueued {
		job.snapshot.Status = JobStatusCancelled
		m.mu.Unlock()
		return nil
	}
	job.snapshot.Status = JobStatusCancelling
	cancel := job.cancel
	m.mu.Unlock()
	cancel()
	return nil
}

func (m *JobManager) ResolveConflict(resolution ConflictResolution) *backend.RPCError {
	m.mu.Lock()
	job := m.jobs[resolution.JobID]
	if job == nil {
		m.mu.Unlock()
		return &backend.RPCError{Code: ErrorJobNotFound, Message: "job not found: " + resolution.JobID}
	}
	if job.snapshot.Status != JobStatusWaitingConflict {
		m.mu.Unlock()
		return &backend.RPCError{Code: ErrorJobConflictRequired, Message: "job is not waiting for conflict: " + resolution.JobID}
	}
	m.mu.Unlock()
	job.conflictCh <- resolution
	return nil
}

func (m *JobManager) run() {
	for job := range m.queue {
		if m.status(job.snapshot.ID) == JobStatusCancelled {
			continue
		}
		m.update(job.snapshot.ID, func(snapshot *JobSnapshot) {
			snapshot.Status = JobStatusRunning
		})
		err := m.runJob(job)
		if err != nil {
			m.update(job.snapshot.ID, func(snapshot *JobSnapshot) {
				if snapshot.Status == JobStatusCancelling {
					snapshot.Status = JobStatusCancelled
					return
				}
				snapshot.Status = JobStatusFailed
				snapshot.Error = &backendError{Code: ErrorFileOperationFailed, Message: err.Error()}
			})
			continue
		}
		m.update(job.snapshot.ID, func(snapshot *JobSnapshot) {
			if snapshot.Status != JobStatusCancelled {
				snapshot.Status = JobStatusCompleted
				snapshot.Conflict = nil
			}
		})
	}
}

func (m *JobManager) runJob(job *fileJob) error {
	switch job.req.Kind {
	case JobKindRename:
		return m.runRename(job)
	case JobKindCopy:
		return m.runCopyMove(job, false)
	case JobKindMove:
		return m.runCopyMove(job, true)
	case JobKindDelete:
		return m.runDelete(job)
	default:
		return fmt.Errorf("unsupported job kind: %s", job.req.Kind)
	}
}

func (m *JobManager) runRename(job *fileJob) error {
	source := job.req.Sources[0]
	target := filepath.Join(filepath.Dir(source), job.req.NewName)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("target already exists: %s", target)
	}
	if err := os.Rename(source, target); err != nil {
		return err
	}
	m.incrementCompleted(job.snapshot.ID)
	return nil
}

func (m *JobManager) runCopyMove(job *fileJob, move bool) error {
	for _, source := range job.req.Sources {
		target := filepath.Join(job.req.TargetDir, filepath.Base(source))
		if _, err := os.Stat(target); err == nil {
			resolution, err := m.waitConflict(job, source, target)
			if err != nil {
				return err
			}
			if resolution.ApplyToAll {
				job.applyAll = &resolution
			}
			switch resolution.Action {
			case ConflictActionSkip:
				m.incrementSkipped(job.snapshot.ID)
				continue
			case ConflictActionKeepBoth:
				target = uniqueCopyTarget(target)
			case ConflictActionOverwrite:
				if err := os.RemoveAll(target); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported conflict action: %s", resolution.Action)
			}
		}
		if err := copyPath(source, target); err != nil {
			return err
		}
		if move {
			if err := os.RemoveAll(source); err != nil {
				return err
			}
		}
		m.incrementCompleted(job.snapshot.ID)
	}
	return nil
}

func (m *JobManager) runDelete(job *fileJob) error {
	for _, source := range job.req.Sources {
		var err error
		if job.req.Permanent {
			err = os.RemoveAll(source)
		} else {
			err = recyclePath(source)
		}
		if err != nil {
			return err
		}
		m.incrementCompleted(job.snapshot.ID)
	}
	return nil
}

func (m *JobManager) waitConflict(job *fileJob, source string, target string) (ConflictResolution, error) {
	if job.applyAll != nil {
		return *job.applyAll, nil
	}
	m.update(job.snapshot.ID, func(snapshot *JobSnapshot) {
		snapshot.Status = JobStatusWaitingConflict
		snapshot.Conflict = &ConflictState{Source: source, Target: target}
	})
	resolution := <-job.conflictCh
	m.update(job.snapshot.ID, func(snapshot *JobSnapshot) {
		snapshot.Status = JobStatusRunning
		snapshot.Conflict = nil
	})
	return resolution, nil
}

func (m *JobManager) status(id string) JobStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[id]; job != nil {
		return job.snapshot.Status
	}
	return ""
}

func (m *JobManager) update(id string, update func(snapshot *JobSnapshot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[id]; job != nil {
		update(&job.snapshot)
	}
}

func (m *JobManager) incrementCompleted(id string) {
	m.update(id, func(snapshot *JobSnapshot) {
		snapshot.Completed++
	})
}

func (m *JobManager) incrementSkipped(id string) {
	m.update(id, func(snapshot *JobSnapshot) {
		snapshot.Completed++
		snapshot.Skipped++
	})
}

func cloneJobSnapshot(snapshot JobSnapshot) JobSnapshot {
	snapshot.Sources = append([]string(nil), snapshot.Sources...)
	if snapshot.Error != nil {
		err := *snapshot.Error
		snapshot.Error = &err
	}
	if snapshot.Conflict != nil {
		conflict := *snapshot.Conflict
		snapshot.Conflict = &conflict
	}
	return snapshot
}

func copyPath(source string, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(source, target)
	}
	return copyFile(source, target, info.Mode())
}

func copyDir(source string, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return copyFile(path, dest, info.Mode())
	})
}

func copyFile(source string, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func uniqueCopyTarget(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	candidate := base + " - Copy" + ext
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = fmt.Sprintf("%s - Copy (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func recyclePath(path string) error {
	from, err := syscall.UTF16FromString(path + "\x00")
	if err != nil {
		return err
	}
	op := shFileOpStruct{
		wFunc:  0x0003,
		pFrom:  &from[0],
		fFlags: 0x0040 | 0x0010 | 0x0400,
	}
	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("move to recycle bin failed: %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("move to recycle bin aborted")
	}
	return nil
}

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var procSHFileOperationW = syscall.NewLazyDLL("shell32.dll").NewProc("SHFileOperationW")
