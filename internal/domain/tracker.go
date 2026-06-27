package domain

import "time"

type FileStatus string

const (
	StatusQueued      FileStatus = "queued"
	StatusDownloading FileStatus = "downloading"
	StatusDownloaded  FileStatus = "downloaded"
	StatusUploading   FileStatus = "uploading"
	StatusFinished    FileStatus = "finished"
	StatusFailed      FileStatus = "failed"
)

type FileRecord struct {
	MessageID    int        `json:"message_id"`
	FileID       string     `json:"file_id"`
	ContentHash  string     `json:"content_hash"`
	Source       string     `json:"source"`
	Group        string     `json:"group"`
	Filename     string     `json:"filename"`
	FileSize     int64      `json:"file_size"`
	Type         string     `json:"type"`
	Status         FileStatus `json:"status"`
	DownloadedAt   time.Time  `json:"downloaded_at"`
	DownloadDoneAt *time.Time `json:"download_done_at,omitempty"`
	UploadStartAt  *time.Time `json:"upload_start_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	UploadedTo     string     `json:"uploaded_to"`
	ULPCount       int        `json:"ulp_count"`
	Error          string     `json:"error,omitempty"`
	FailedAt       *time.Time `json:"failed_at,omitempty"`
	Password     string     `json:"password,omitempty"`
}

type GroupState struct {
	LastMessageID   int       `json:"last_message_id"`
	OldestMessageID int       `json:"oldest_message_id"`
	LastCheck       time.Time `json:"last_check"`
	TotalDownloaded int       `json:"total_downloaded"`
	TotalULPs       int       `json:"total_ulps"`
	TotalFailed     int       `json:"total_failed"`
	ConsecFails     int       `json:"consec_fails"` // falhas consecutivas de validação
	LastValidated   time.Time `json:"last_validated"`
}

type TrackerState struct {
	Groups          map[string]GroupState `json:"groups"`
	DownloadedFiles map[string]FileRecord `json:"downloaded_files"`
	Pending         []PendingFile         `json:"pending"`
}

type PendingFile struct {
	MessageID int       `json:"message_id"`
	FileID    string    `json:"file_id"`
	Source    string    `json:"source"`
	Group     string    `json:"group"`
	Filename  string    `json:"filename"`
	FileSize  int64     `json:"file_size"`
	Date      time.Time `json:"date"`
	Priority  int       `json:"priority"`
	Password  string    `json:"password"`
}
