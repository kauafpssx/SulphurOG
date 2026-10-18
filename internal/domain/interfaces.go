package domain

import (
	"context"
	"time"
)

type TelegramClient interface {
	Connect(ctx context.Context) error
	ResolveChannel(identifier string) (active bool, channelID int64, accessHash int64, err error)
	GetMessages(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]LogFile, error)
	ListFiles(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]LogFile, error)
	DownloadFile(ctx context.Context, location interface{}, destPath string, totalSize int64, threads int) (int64, error)
	GetChannelStatus(ctx context.Context, identifier string) (bool, int, time.Time, error)
	Disconnect()
}

type ArchiveExtractor interface {
	Extract(archivePath string, password string, destDir string) ([]string, error)
	DetectType(filePath string) string
}

type LogParser interface {
	ParseFile(filePath string) (*ExtractedData, error)
	ParsePasswords(content string) []ULP
	ParseCookies(content string) []Cookie
	IsULPFormat(content []byte) bool
}

// ArtifactStorage persists processed artifacts. Implementations must return a
// stable path that can be stored in the tracker.
type ArtifactStorage interface {
	Write(path string, data []byte) (string, error)
}

type GroupRepository interface {
	GetAll() ([]Group, error)
	GetByID(id string) (*Group, error)
	Create(group *Group) error
	Update(group *Group) error
	Delete(id string) error
}

type Tracker interface {
	Load() (*TrackerState, error)
	Save(state *TrackerState) error
	IsDownloaded(contentHash string) (bool, error)
	GetFileRecord(contentHash string) (*FileRecord, error)
	MarkDownloaded(record FileRecord) error
	MarkDownloadComplete(contentHash string) error
	MarkUploading(contentHash string) error
	MarkFinished(contentHash string, uploadedPath string, ulpCount int) error
	MarkFailed(contentHash string, errMsg string) error
	MarkPermanentFailed(contentHash string, errMsg string) error
	GetPending(limit int) ([]PendingFile, error)
	AddPending(files []PendingFile) error
	RemovePending(sourceURL string) error
	GetGroupState(groupURL string) (*GroupState, error)
	UpdateGroupState(groupURL string, state GroupState) error
	RemovePendingByGroup(groupID string) error
	IsDuplicateFile(filename string, fileSize int64) (bool, error)
	IsDuplicateMessage(filename string, messageID int) (bool, error)
	GetStats() (downloaded int, processed int, failed int, pending int, err error)
	GetDetailedStats() (*DetailedStats, error)
}

type HashService interface {
	HashFile(filePath string) (string, error)
	HashBytes(data []byte) string
}
