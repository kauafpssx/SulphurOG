# Tecnologias e Bibliotecas

## Linguagem

**Go 1.22+** — Compilacao estatica, binario unico, baixo consumo de memoria

## Dependencias

| Biblioteca | Uso |
|-----------|-----|
| `github.com/gofiber/fiber/v2` | HTTP API leve |
| `github.com/gotd/td` | MTProto 2.0, downloads via CDN |
| `github.com/alexmullins/zip` | ZIP com senha (puro Go) |
| `github.com/rs/zerolog` | Logs estruturados |
| `gopkg.in/yaml.v3` | Configuracao |

## Ferramentas Externas

| Ferramenta | Uso | Local |
|-----------|-----|-------|
| 7z | Extracao RAR/7z | `7-ZIP/7z.exe` (portable, incluido) |
| unrar | Extracao RAR (fallback) | PATH do sistema |

## Variaveis de Ambiente (.env)

```bash
# Telegram
TG_API_ID=12345678
TG_API_HASH=abcdef1234567890
TG_PHONE=+5511999999999

# Supabase
SUPABASE_URL=https://xxxxx.supabase.co
SUPABASE_ANON_KEY=eyJ...
SUPABASE_SERVICE_ROLE_KEY=eyJ...
SUPABASE_BUCKET=sulphurog-logs

# API
API_KEY=teste
API_PORT=8090

# Paths
DATA_DIR=data
LOG_DIR=logs
```

## Cross-Compile

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/sulphurog-linux ./cmd/sulphurog/
```
