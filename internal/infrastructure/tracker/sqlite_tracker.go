package tracker

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type SQLiteTracker struct {
	db *sql.DB
}

func NewSQLiteTracker(filePath string) (*SQLiteTracker, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", filePath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	t := &SQLiteTracker{db: db}
	if err := t.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return t, nil
}

func (t *SQLiteTracker) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS downloaded_files (
			content_hash    TEXT PRIMARY KEY,
			message_id      INTEGER NOT NULL DEFAULT 0,
			file_id         TEXT NOT NULL DEFAULT '',
			source          TEXT NOT NULL DEFAULT '',
			group_id        TEXT NOT NULL DEFAULT '',
			filename        TEXT NOT NULL DEFAULT '',
			file_size       INTEGER NOT NULL DEFAULT 0,
			type            TEXT NOT NULL DEFAULT '',
			status          TEXT NOT NULL DEFAULT 'queued',
			downloaded_at   TEXT,
			download_done_at TEXT,
			upload_start_at TEXT,
			finished_at     TEXT,
			failed_at       TEXT,
			uploaded_to     TEXT NOT NULL DEFAULT '',
			ulp_count       INTEGER NOT NULL DEFAULT 0,
			error           TEXT NOT NULL DEFAULT '',
			password        TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS pending_files (
			source     TEXT PRIMARY KEY,
			message_id INTEGER NOT NULL DEFAULT 0,
			file_id    TEXT NOT NULL DEFAULT '',
			group_id   TEXT NOT NULL DEFAULT '',
			filename   TEXT NOT NULL DEFAULT '',
			file_size  INTEGER NOT NULL DEFAULT 0,
			date       TEXT,
			priority   INTEGER NOT NULL DEFAULT 1,
			password   TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS group_states (
			group_url         TEXT PRIMARY KEY,
			last_message_id   INTEGER NOT NULL DEFAULT 0,
			oldest_message_id INTEGER NOT NULL DEFAULT 0,
			last_check        TEXT,
			total_downloaded  INTEGER NOT NULL DEFAULT 0,
			total_ulps        INTEGER NOT NULL DEFAULT 0,
			total_failed      INTEGER NOT NULL DEFAULT 0,
			consec_fails      INTEGER NOT NULL DEFAULT 0,
			last_validated    TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_priority ON pending_files(priority ASC, date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_downloaded_status ON downloaded_files(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_group ON pending_files(group_id)`,
	}
	for _, stmt := range stmts {
		if _, err := t.db.Exec(stmt); err != nil {
			return err
		}
	}
	// Migrações para DBs existentes (ignora se coluna já existe)
	migrations := []string{
		`ALTER TABLE group_states ADD COLUMN consec_fails INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE group_states ADD COLUMN last_validated TEXT`,
	}
	for _, m := range migrations {
		if _, err := t.db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// Load reconstrói TrackerState completo do banco (compatibilidade com interface)
func (t *SQLiteTracker) Load() (*domain.TrackerState, error) {
	state := &domain.TrackerState{
		Groups:          make(map[string]domain.GroupState),
		DownloadedFiles: make(map[string]domain.FileRecord),
		Pending:         []domain.PendingFile{},
	}

	rows, err := t.db.Query(`SELECT content_hash, message_id, file_id, source, group_id, filename, file_size, type, status,
		downloaded_at, download_done_at, upload_start_at, finished_at, failed_at, uploaded_to, ulp_count, error, password
		FROM downloaded_files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r domain.FileRecord
		var downloadedAt, downloadDoneAt, uploadStartAt, finishedAt, failedAt string
		if err := rows.Scan(&r.ContentHash, &r.MessageID, &r.FileID, &r.Source, &r.Group, &r.Filename, &r.FileSize,
			&r.Type, &r.Status, &downloadedAt, &downloadDoneAt, &uploadStartAt, &finishedAt, &failedAt,
			&r.UploadedTo, &r.ULPCount, &r.Error, &r.Password); err != nil {
			continue
		}
		r.DownloadedAt = parseTime(downloadedAt)
		r.DownloadDoneAt = parseNullTime(downloadDoneAt)
		r.UploadStartAt = parseNullTime(uploadStartAt)
		r.FinishedAt = parseNullTime(finishedAt)
		r.FailedAt = parseNullTime(failedAt)
		state.DownloadedFiles[r.ContentHash] = r
	}

	pendingRows, err := t.db.Query(`SELECT source, message_id, file_id, group_id, filename, file_size, date, priority, password FROM pending_files ORDER BY priority ASC, date DESC`)
	if err != nil {
		return nil, err
	}
	defer pendingRows.Close()
	for pendingRows.Next() {
		var p domain.PendingFile
		var dateStr string
		if err := pendingRows.Scan(&p.Source, &p.MessageID, &p.FileID, &p.Group, &p.Filename, &p.FileSize, &dateStr, &p.Priority, &p.Password); err != nil {
			continue
		}
		p.Date = parseTime(dateStr)
		state.Pending = append(state.Pending, p)
	}

	groupRows, err := t.db.Query(`SELECT group_url, last_message_id, oldest_message_id, last_check, total_downloaded, total_ulps, total_failed FROM group_states`)
	if err != nil {
		return nil, err
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var g domain.GroupState
		var groupURL, lastCheck string
		if err := groupRows.Scan(&groupURL, &g.LastMessageID, &g.OldestMessageID, &lastCheck, &g.TotalDownloaded, &g.TotalULPs, &g.TotalFailed); err != nil {
			continue
		}
		g.LastCheck = parseTime(lastCheck)
		state.Groups[groupURL] = g
	}

	return state, nil
}

// Save faz upsert completo (compatibilidade com interface)
func (t *SQLiteTracker) Save(state *domain.TrackerState) error {
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, r := range state.DownloadedFiles {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO downloaded_files VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			r.ContentHash, r.MessageID, r.FileID, r.Source, r.Group, r.Filename, r.FileSize, r.Type, r.Status,
			formatTime(r.DownloadedAt), formatNullTime(r.DownloadDoneAt), formatNullTime(r.UploadStartAt),
			formatNullTime(r.FinishedAt), formatNullTime(r.FailedAt),
			r.UploadedTo, r.ULPCount, r.Error, r.Password); err != nil {
			return err
		}
	}
	for _, p := range state.Pending {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO pending_files VALUES (?,?,?,?,?,?,?,?,?)`,
			p.Source, p.MessageID, p.FileID, p.Group, p.Filename, p.FileSize, formatTime(p.Date), p.Priority, p.Password); err != nil {
			return err
		}
	}
	for groupURL, g := range state.Groups {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO group_states VALUES (?,?,?,?,?,?,?)`,
			groupURL, g.LastMessageID, g.OldestMessageID, formatTime(g.LastCheck), g.TotalDownloaded, g.TotalULPs, g.TotalFailed); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (t *SQLiteTracker) IsDownloaded(contentHash string) (bool, error) {
	var count int
	err := t.db.QueryRow(`SELECT COUNT(*) FROM downloaded_files WHERE content_hash = ? AND status IN ('finished','failed')`, contentHash).Scan(&count)
	return count > 0, err
}

func (t *SQLiteTracker) GetFileRecord(contentHash string) (*domain.FileRecord, error) {
	var r domain.FileRecord
	var downloadedAt, downloadDoneAt, uploadStartAt, finishedAt, failedAt string
	err := t.db.QueryRow(`SELECT content_hash, message_id, file_id, source, group_id, filename, file_size, type, status,
		downloaded_at, download_done_at, upload_start_at, finished_at, failed_at, uploaded_to, ulp_count, error, password
		FROM downloaded_files WHERE content_hash = ?`, contentHash).
		Scan(&r.ContentHash, &r.MessageID, &r.FileID, &r.Source, &r.Group, &r.Filename, &r.FileSize,
			&r.Type, &r.Status, &downloadedAt, &downloadDoneAt, &uploadStartAt, &finishedAt, &failedAt,
			&r.UploadedTo, &r.ULPCount, &r.Error, &r.Password)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file not found: %s", contentHash)
	}
	if err != nil {
		return nil, err
	}
	r.DownloadedAt = parseTime(downloadedAt)
	r.DownloadDoneAt = parseNullTime(downloadDoneAt)
	r.UploadStartAt = parseNullTime(uploadStartAt)
	r.FinishedAt = parseNullTime(finishedAt)
	r.FailedAt = parseNullTime(failedAt)
	return &r, nil
}

func (t *SQLiteTracker) MarkDownloaded(record domain.FileRecord) error {
	_, err := t.db.Exec(`INSERT OR REPLACE INTO downloaded_files
		(content_hash, message_id, file_id, source, group_id, filename, file_size, password, status, downloaded_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		record.ContentHash, record.MessageID, record.FileID, record.Source, record.Group,
		record.Filename, record.FileSize, record.Password,
		domain.StatusDownloading, formatTime(time.Now()))
	return err
}

func (t *SQLiteTracker) MarkDownloadComplete(contentHash string) error {
	_, err := t.db.Exec(`UPDATE downloaded_files SET status = ?, download_done_at = ? WHERE content_hash = ?`,
		domain.StatusDownloaded, formatTime(time.Now()), contentHash)
	return err
}

func (t *SQLiteTracker) MarkUploading(contentHash string) error {
	_, err := t.db.Exec(`UPDATE downloaded_files SET status = ?, upload_start_at = ? WHERE content_hash = ?`,
		domain.StatusUploading, formatTime(time.Now()), contentHash)
	return err
}

func (t *SQLiteTracker) MarkFinished(contentHash, uploadedPath string, ulpCount int) error {
	now := formatTime(time.Now())
	_, err := t.db.Exec(`UPDATE downloaded_files SET status = ?, uploaded_to = ?, ulp_count = ?, finished_at = ? WHERE content_hash = ?`,
		domain.StatusFinished, uploadedPath, ulpCount, now, contentHash)
	if err != nil {
		return err
	}
	// Atualiza stats do grupo
	t.db.Exec(`UPDATE group_states SET total_downloaded = total_downloaded + 1, total_ulps = total_ulps + ?
		WHERE group_url = (SELECT group_id FROM downloaded_files WHERE content_hash = ?)`,
		ulpCount, contentHash)
	return nil
}

func (t *SQLiteTracker) MarkFailed(contentHash, errMsg string) error {
	now := formatTime(time.Now())
	_, err := t.db.Exec(`UPDATE downloaded_files SET status = ?, error = ?, failed_at = ? WHERE content_hash = ?`,
		domain.StatusFailed, errMsg, now, contentHash)
	if err != nil {
		return err
	}
	t.db.Exec(`UPDATE group_states SET total_failed = total_failed + 1
		WHERE group_url = (SELECT group_id FROM downloaded_files WHERE content_hash = ?)`, contentHash)
	return nil
}

func (t *SQLiteTracker) GetPending(limit int) ([]domain.PendingFile, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := t.db.Query(`SELECT source, message_id, file_id, group_id, filename, file_size, date, priority, password
		FROM pending_files ORDER BY priority ASC, date DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.PendingFile
	for rows.Next() {
		var p domain.PendingFile
		var dateStr string
		if err := rows.Scan(&p.Source, &p.MessageID, &p.FileID, &p.Group, &p.Filename, &p.FileSize, &dateStr, &p.Priority, &p.Password); err != nil {
			continue
		}
		p.Date = parseTime(dateStr)
		result = append(result, p)
	}
	return result, nil
}

func (t *SQLiteTracker) AddPending(files []domain.PendingFile) error {
	tx, err := t.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, f := range files {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO pending_files (source, message_id, file_id, group_id, filename, file_size, date, priority, password) VALUES (?,?,?,?,?,?,?,?,?)`,
			f.Source, f.MessageID, f.FileID, f.Group, f.Filename, f.FileSize, formatTime(f.Date), f.Priority, f.Password); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (t *SQLiteTracker) RemovePending(sourceURL string) error {
	_, err := t.db.Exec(`DELETE FROM pending_files WHERE source = ?`, sourceURL)
	return err
}

func (t *SQLiteTracker) GetGroupState(groupURL string) (*domain.GroupState, error) {
	var g domain.GroupState
	var lastCheck, lastValidated string
	err := t.db.QueryRow(`SELECT last_message_id, oldest_message_id, last_check, total_downloaded, total_ulps, total_failed, consec_fails, last_validated
		FROM group_states WHERE group_url = ?`, groupURL).
		Scan(&g.LastMessageID, &g.OldestMessageID, &lastCheck, &g.TotalDownloaded, &g.TotalULPs, &g.TotalFailed, &g.ConsecFails, &lastValidated)
	if err == sql.ErrNoRows {
		return &domain.GroupState{}, nil
	}
	if err != nil {
		return nil, err
	}
	g.LastCheck = parseTime(lastCheck)
	g.LastValidated = parseTime(lastValidated)
	return &g, nil
}

func (t *SQLiteTracker) UpdateGroupState(groupURL string, state domain.GroupState) error {
	state.LastCheck = time.Now()
	_, err := t.db.Exec(`INSERT OR REPLACE INTO group_states
		(group_url, last_message_id, oldest_message_id, last_check, total_downloaded, total_ulps, total_failed, consec_fails, last_validated)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		groupURL, state.LastMessageID, state.OldestMessageID, formatTime(state.LastCheck),
		state.TotalDownloaded, state.TotalULPs, state.TotalFailed, state.ConsecFails, formatTime(state.LastValidated))
	return err
}

func (t *SQLiteTracker) RemovePendingByGroup(groupID string) error {
	_, err := t.db.Exec(`DELETE FROM pending_files WHERE group_id = ?`, groupID)
	return err
}

func (t *SQLiteTracker) IsDuplicateFile(filename string, fileSize int64) (bool, error) {
	var count int
	err := t.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM pending_files WHERE filename = ? AND file_size = ?
			UNION ALL
			SELECT 1 FROM downloaded_files WHERE filename = ? AND file_size = ? AND status = 'finished'
		)`, filename, fileSize, filename, fileSize).Scan(&count)
	return count > 0, err
}

func (t *SQLiteTracker) GetStats() (downloading, finished, failed, pending int, err error) {
	t.db.QueryRow(`SELECT COUNT(*) FROM downloaded_files WHERE status IN ('downloading','downloaded','uploading')`).Scan(&downloading)
	t.db.QueryRow(`SELECT COUNT(*) FROM downloaded_files WHERE status = 'finished'`).Scan(&finished)
	t.db.QueryRow(`SELECT COUNT(*) FROM downloaded_files WHERE status = 'failed'`).Scan(&failed)
	t.db.QueryRow(`SELECT COUNT(*) FROM pending_files`).Scan(&pending)
	return
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatNullTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseNullTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
