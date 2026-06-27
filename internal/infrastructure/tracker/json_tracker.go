package tracker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type JSONTracker struct {
	filePath string
	mu       sync.RWMutex
	state    *domain.TrackerState
}

func NewJSONTracker(filePath string) (*JSONTracker, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tracker dir: %w", err)
	}

	t := &JSONTracker{
		filePath: filePath,
		state: &domain.TrackerState{
			Groups:          make(map[string]domain.GroupState),
			DownloadedFiles: make(map[string]domain.FileRecord),
			Pending:         []domain.PendingFile{},
		},
	}

	if err := t.load(); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *JSONTracker) load() error {
	data, err := os.ReadFile(t.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read tracker: %w", err)
	}

	return json.Unmarshal(data, t.state)
}

func (t *JSONTracker) Load() (*domain.TrackerState, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state, nil
}

func (t *JSONTracker) Save(state *domain.TrackerState) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = state
	return t.save()
}

func (t *JSONTracker) save() error {
	data, err := json.MarshalIndent(t.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker: %w", err)
	}

	// Atomic write
	tmpPath := t.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracker tmp: %w", err)
	}

	if err := os.Rename(tmpPath, t.filePath); err != nil {
		return fmt.Errorf("failed to rename tracker: %w", err)
	}

	// Backup
	backupPath := t.filePath + ".bak"
	if _, err := os.Stat(t.filePath); err == nil {
		src, err := os.Open(t.filePath)
		if err == nil {
			defer src.Close()
			dst, _ := os.Create(backupPath)
			if dst != nil {
				defer dst.Close()
				scanner := bufio.NewScanner(src)
				writer := bufio.NewWriter(dst)
				for scanner.Scan() {
					writer.WriteString(scanner.Text() + "\n")
				}
				writer.Flush()
			}
		}
	}

	return nil
}

func (t *JSONTracker) IsDownloaded(contentHash string) (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return false, nil
	}
	return record.Status == domain.StatusFinished || record.Status == domain.StatusFailed, nil
}

func (t *JSONTracker) GetFileRecord(contentHash string) (*domain.FileRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", contentHash)
	}
	return &record, nil
}

func (t *JSONTracker) MarkDownloaded(record domain.FileRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record.Status = domain.StatusDownloading
	record.DownloadedAt = time.Now()
	t.state.DownloadedFiles[record.ContentHash] = record

	return t.save()
}

func (t *JSONTracker) MarkDownloadComplete(contentHash string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return fmt.Errorf("file not found: %s", contentHash)
	}

	now := time.Now()
	record.Status = domain.StatusDownloaded
	record.DownloadDoneAt = &now
	t.state.DownloadedFiles[contentHash] = record

	return t.save()
}

func (t *JSONTracker) MarkUploading(contentHash string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return fmt.Errorf("file not found: %s", contentHash)
	}

	now := time.Now()
	record.Status = domain.StatusUploading
	record.UploadStartAt = &now
	t.state.DownloadedFiles[contentHash] = record

	return t.save()
}

func (t *JSONTracker) MarkFinished(contentHash string, uploadedPath string, ulpCount int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return fmt.Errorf("file not found: %s", contentHash)
	}

	now := time.Now()
	record.Status = domain.StatusFinished
	record.UploadedTo = uploadedPath
	record.ULPCount = ulpCount
	record.FinishedAt = &now
	t.state.DownloadedFiles[contentHash] = record

	// Atualizar stats do grupo
	if group, ok := t.state.Groups[record.Group]; ok {
		group.TotalDownloaded++
		group.TotalULPs += ulpCount
		t.state.Groups[record.Group] = group
	}

	return t.save()
}

func (t *JSONTracker) MarkFailed(contentHash string, errMsg string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, exists := t.state.DownloadedFiles[contentHash]
	if !exists {
		return fmt.Errorf("file not found: %s", contentHash)
	}

	now := time.Now()
	record.Status = domain.StatusFailed
	record.Error = errMsg
	record.FailedAt = &now
	t.state.DownloadedFiles[contentHash] = record

	// Atualizar stats do grupo
	if group, ok := t.state.Groups[record.Group]; ok {
		group.TotalFailed++
		t.state.Groups[record.Group] = group
	}

	return t.save()
}

func (t *JSONTracker) GetPending(limit int) ([]domain.PendingFile, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.state.Pending) {
		limit = len(t.state.Pending)
	}

	// Ordenar por prioridade (1 primeiro) e data (recente primeiro)
	pending := make([]domain.PendingFile, len(t.state.Pending))
	copy(pending, t.state.Pending)

	// Sort simples: prioridade 1 primeiro, depois por data desc
	for i := 0; i < len(pending); i++ {
		for j := i + 1; j < len(pending); j++ {
			if pending[j].Priority < pending[i].Priority ||
				(pending[j].Priority == pending[i].Priority && pending[j].Date.After(pending[i].Date)) {
				pending[i], pending[j] = pending[j], pending[i]
			}
		}
	}

	return pending[:limit], nil
}

func (t *JSONTracker) AddPending(files []domain.PendingFile) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Nao adicionar duplicatas
	existing := make(map[string]bool)
	for _, p := range t.state.Pending {
		existing[p.Source] = true
	}

	for _, f := range files {
		if !existing[f.Source] {
			t.state.Pending = append(t.state.Pending, f)
		}
	}

	return t.save()
}

func (t *JSONTracker) RemovePendingByGroup(groupID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	filtered := t.state.Pending[:0]
	for _, p := range t.state.Pending {
		if p.Group != groupID {
			filtered = append(filtered, p)
		}
	}
	t.state.Pending = filtered
	return t.save()
}

func (t *JSONTracker) RemovePending(sourceURL string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, p := range t.state.Pending {
		if p.Source == sourceURL {
			t.state.Pending = append(t.state.Pending[:i], t.state.Pending[i+1:]...)
			break
		}
	}

	return t.save()
}

func (t *JSONTracker) GetGroupState(groupURL string) (*domain.GroupState, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state, exists := t.state.Groups[groupURL]
	if !exists {
		return nil, fmt.Errorf("group state not found: %s", groupURL)
	}
	return &state, nil
}

func (t *JSONTracker) UpdateGroupState(groupURL string, state domain.GroupState) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	state.LastCheck = time.Now()
	t.state.Groups[groupURL] = state

	return t.save()
}

func (t *JSONTracker) GetStats() (downloading int, finished int, failed int, pending int, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, record := range t.state.DownloadedFiles {
		switch record.Status {
		case domain.StatusDownloading, domain.StatusDownloaded, domain.StatusUploading:
			downloading++
		case domain.StatusFinished:
			finished++
		case domain.StatusFailed:
			failed++
		}
	}
	pending = len(t.state.Pending)

	return
}
