# Arquitetura do Sistema

## Clean Architecture

```
+---------------------------------------------------+
|              Frameworks & Drivers                  |
|  (Fiber HTTP, Telegram MTProto, Supabase REST)    |
|  +-----------------------------------------------+ |
|  |           Adapters / Infrastructure            | |
|  |  (telegram, supabase, extractor, parser)       | |
|  |  +-------------------------------------------+ | |
|  |  |           Use Cases / Application           | | |
|  |  |  (ProcessFile, MonitorGroups, ManageGroups) | | |
|  |  |  +---------------------------------------+ | | |
|  |  |  |           Domain / Entities            | | | |
|  |  |  |  (Group, ULP, LogFile, TrackerState)   | | | |
|  |  |  +---------------------------------------+ | | |
|  |  +-------------------------------------------+ | |
|  +-----------------------------------------------+ |
+---------------------------------------------------+
```

## Interfaces (Portas)

```go
type TelegramClient interface {
    Connect(ctx context.Context) error
    ResolveChannel(identifier string) (active bool, channelID int64, accessHash int64, err error)
    GetMessages(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]LogFile, error)
    ListFiles(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]LogFile, error)
    DownloadFile(ctx context.Context, location interface{}, destPath string, totalSize int64) (int64, error)
    GetChannelStatus(ctx context.Context, identifier string) (bool, int, time.Time, error)
    Disconnect()
}

type ArchiveExtractor interface {
    Extract(archivePath, password, destDir string) ([]string, error)
    DetectType(filePath string) string
}

type LogParser interface {
    ParsePasswords(content string) []ULP
    ParseCookies(content string) []Cookie
    IsULPFormat(content []byte) bool
}

type SupabaseStorage interface {
    Upload(ctx context.Context, bucket, path string, data []byte) error
    UploadReader(ctx context.Context, bucket, path string, reader io.Reader, size int64) error
}

type GroupRepository interface {
    GetAll() ([]Group, error)
    GetByID(id string) (*Group, error)
    Create(group *Group) error
    Update(group *Group) error
    Delete(id string) error
}

type Tracker interface {
    IsDownloaded(contentHash string) (bool, error)
    MarkDownloaded(record FileRecord) error
    MarkFinished(contentHash, uploadedPath string, ulpCount int) error
    MarkFailed(contentHash, errMsg string) error
    GetPending(limit int) ([]PendingFile, error)
    AddPending(files []PendingFile) error
    RemovePending(sourceURL string) error
    GetGroupState(groupURL string) (*GroupState, error)
    UpdateGroupState(groupURL string, state GroupState) error
    GetStats() (downloaded, processed, failed, pending int, err error)
}

type HashService interface {
    HashFile(filePath string) (string, error)
    HashBytes(data []byte) string
}
```

## Fluxo de Dependencias

```
cmd/sulphurog/main.go
    ├── api/server.go           (Fiber HTTP)
    │   ├── api/groups.go       (handlers CRUD)
    │   ├── api/monitor.go      (start/pause/status)
    │   └── api/middleware.go   (X-API-Key auth)
    ├── usecase/
    │   ├── process_file.go    (download→detect→extract→parse→upload)
    │   ├── monitor_groups.go  (loop sequencial 1 arquivo por vez)
    │   └── manage_groups.go   (CRUD + health check)
    └── infrastructure/
        ├── telegram/gotd_client.go   (MTProto, downloads, progress bar)
        ├── extractor/
        │   ├── detector.go           (magic bytes)
        │   ├── zip_extractor.go      (ZIP com senha)
        │   ├── sevenz_extractor.go   (RAR/7z via 7z CLI)
        │   └── sevenz.go             (wrapper 7z portable)
        ├── parser/stealer.go         (YAML, Key-Value, ULP direto)
        ├── supabase/client.go        (REST upload)
        ├── tracker/json_tracker.go   (downloaded.json)
        ├── repository/json_group_repo.go (grupos em JSON)
        └── hash/sha256.go           (deduplicacao)
```
