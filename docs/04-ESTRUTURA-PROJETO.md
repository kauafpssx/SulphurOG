# Estrutura do Projeto

```
sulphurog/
├── cmd/
│   ├── sulphurog/          # Ponto de entrada principal
│   │   ├── main.go         # Wire de dependencias, graceful shutdown
│   │   └── config.go       # Config YAML + .env loader
│   └── auth/               # Tool de autenticacao Telegram
│       └── main.go
├── internal/
│   ├── domain/             # Structs + interfaces (zero deps)
│   │   ├── group.go        # Group + normalizacao URL/ID
│   │   ├── log_file.go     # LogFile + FormatSize
│   │   ├── ulp.go          # ULP + String()
│   │   ├── extracted_data.go # ExtractedData, Cookie
│   │   ├── tracker.go      # TrackerState, FileRecord, PendingFile
│   │   └── interfaces.go   # Todas as interfaces
│   ├── usecase/            # Orquestracao
│   │   ├── process_file.go # Download→detect→extract→parse→upload
│   │   ├── monitor_groups.go # Loop sequencial 1 arquivo por vez
│   │   └── manage_groups.go # CRUD + health check
│   └── infrastructure/     # Implementacoes
│       ├── api/
│       │   ├── groups_handler.go   # CRUD handlers
│       │   ├── monitor_handler.go  # start/pause/status
│       │   ├── health_handler.go   # health endpoints
│       │   ├── status_handler.go   # status geral
│       │   ├── middleware.go       # X-API-Key auth
│       │   └── progress.go        # Progress bar helper
│       ├── telegram/
│       │   └── gotd_client.go     # MTProto, downloads, progress bar
│       ├── extractor/
│       │   ├── detector.go        # Magic bytes
│       │   ├── zip_extractor.go   # ZIP com senha
│       │   ├── sevenz_extractor.go # RAR/7z via CLI
│       │   └── sevenz.go          # Wrapper 7z portable
│       ├── parser/
│       │   └── stealer.go         # YAML, Key-Value, ULP direto
│       ├── supabase/
│       │   └── client.go          # REST upload
│       ├── tracker/
│       │   └── json_tracker.go    # downloaded.json
│       ├── repository/
│       │   └── json_group_repo.go # Grupos em JSON
│       └── hash/
│           └── sha256.go          # Deduplicacao
├── 7-ZIP/                         # 7z portable (Windows)
├── configs/
│   └── config.yaml
├── scripts/                       # Scripts de teste
├── docs/                          # Documentacao
├── data/                          # Dados persistidos
│   ├── groups.json
│   ├── downloaded.json
│   └── session.json
├── go.mod
└── Makefile
```
