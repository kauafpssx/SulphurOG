# SulphurOG

**A Telegram archive monitor that turns supported log files into searchable local output.**

SulphurOG watches configured Telegram channels with a user account, queues new archives, downloads them one at a time, extracts them, parses text records, and writes results under `data/output/`. Its HTTP API searches saved ULP text by site.

> [!IMPORTANT]
> Output can contain credentials and cookies. Keep `data/`, `.env`, and the API key private. Docker publishes port `8090` on the host; restrict access with your cloud firewall or a trusted network.

## ✨ What it does

| Stage | Behavior |
| --- | --- |
| Discover | Scans active Telegram groups for recent files and advances through older history. |
| Queue | Stores eligible files in SQLite and recognizes files already queued or processed by channel and message. |
| Process | Downloads one file at a time, up to **three per cycle**. Accepts `.zip`, `.rar`, `.7z`, `.gz`, and `.txt` when enabled. |
| Extract | Uses 7-Zip for archives, including password-protected files. The Docker image includes the RAR codec. |
| Save | Writes parsed `ulp.txt` and, when enabled and present, `cookies.zip` to a timestamped local folder. |

The monitor checks for new posts on each cycle. Older files remain in the persistent queue while newer files are processed first. Files with the same name in different messages are separate files. A group can be configured to skip files without a password.

## 🚀 Quick start with Docker

### Requirements

- Docker with Compose
- A Telegram account and API ID/hash from [my.telegram.org](https://my.telegram.org)
- Go **1.26.3** to build the Linux binary locally

```bash
git clone https://github.com/kauafpssx/SulphurOG.git
cd SulphurOG
cp .env.example .env
# Fill in TG_API_ID, TG_API_HASH, and TG_PHONE in .env.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o sulphurog ./cmd/sulphurog
docker compose up -d --build
docker compose logs -f
```

The Dockerfile copies a **prebuilt Linux AMD64 binary** named `sulphurog` from the repository root. Build it before running Compose. The image installs 7-Zip with RAR support.

### Telegram sign-in

The first connection may require a Telegram verification code. Start the container interactively once, using the same `data/` volume as Compose:

```bash
docker compose run --rm sulphurog
```

Enter the code when prompted. The session is saved to `data/session.json`. Stop the interactive run with `Ctrl+C`, then start the service normally:

```bash
docker compose up -d
```

When `API_KEY` is unset, the app generates a key on first start and stores it in `data/api.key`. You may set `API_KEY` in `.env` instead. Do not share either value.

## ⚙️ Configuration

The app reads `configs/config.yaml` and expands Telegram and API values from `.env`.

| Setting | Default in this repository | Purpose |
| --- | --- | --- |
| `api.port` | `8090` | HTTP port published by Compose. |
| `telegram.session_file` | `data/session.json` | Saved Telegram login. |
| `processing.temp_dir` | `/tmp/sulphurog` | Working directory for downloads and extraction. |
| `processing.process_cookies` | `true` | Save parsed cookies in a ZIP. |
| `processing.allowed_extensions` | ZIP, RAR, 7z, GZ, TXT | File types eligible for the queue. |
| `storage.output_dir` | `data/output` | Local results. |

Manage groups from the project directory, where `data/` and `configs/` are available:

```bash
./sulphurog --addgroup "https://t.me/channelname" "Display name"
./sulphurog --refresh
```

`--refresh` marks saved groups for revalidation on the next monitor cycle.

## 📂 Files and state

```text
data/
├── api.key                 # Generated API key, unless API_KEY is set
├── groups.json             # Configured groups
├── session.json            # Telegram session
├── sulphurog.db            # Queue and processing history
└── output/
    └── 2026-11-24_14-39-22/
        ├── ulp.txt         # Parsed URL:login:password lines
        └── cookies.zip     # Present when cookies were found and enabled
```

Keep `sulphurog.db` and `output/` together across deployments. Removing the database resets the queue and processed-file history, so the monitor may download old files again. Compose mounts `./data` into the container, preserving it when the container is rebuilt.

## 🔌 HTTP API

The server listens on port `8090` by default. `GET /api/health` is public. `POST /api/master/files` requires `X-API-Key`.

```bash
curl http://localhost:8090/api/health

curl -X POST http://localhost:8090/api/master/files \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"site":"example.com","file_type":"txt"}'
```

The master route searches saved `.txt` files for lines containing `site` and returns matching URL, login, and password fields as JSON. `file_type` defaults to `txt`. Set `removeafterget` to `yes` **only if you intend to delete matching lines from the saved text files**; the default leaves them intact.

> [!NOTE]
> The `cookies` request value currently scans `.txt` files too. It does not search inside the generated `cookies.zip` files.

## 🧭 How it is organized

```text
cmd/sulphurog/                   Entry point and configuration
internal/domain/                 Data types and interfaces
internal/usecase/                Group monitoring and file processing
internal/infrastructure/telegram Telegram MTProto client
internal/infrastructure/tracker  SQLite queue and history
internal/infrastructure/extractor Archive handling
internal/infrastructure/api/     HTTP routes
```

The runtime uses Go, [gotd/td](https://github.com/gotd/td), Fiber, SQLite, and 7-Zip. GitHub Actions builds the Linux AMD64 binary, copies it and the Docker files to the server, then rebuilds the Compose service on pushes to `main`.

## 🧪 Local checks

```bash
go test ./...
```
