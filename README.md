# 🔥 SulphurOG

Monitora canais do Telegram, baixa logs de stealer, extrai credenciais e salva no Supabase Storage.

## 🚀 O que faz

- Conecta em canais via MTProto (conta real, sem bot)
- Detecta arquivos novos a cada ciclo, enfileira e processa 10 por vez
- Baixa ZIP/RAR/7z com senha, extrai no disco com 7z, lê `.txt` um por um
- Parseia três formatos de log: ULP simples (`URL:login:senha`), YAML e Key-Value
- Agrupa cookies por vítima, sobe `ulp.txt` + `cookies.zip` pro Supabase
- Divide arquivos ULP acima de 49MB automaticamente
- Filtra extensões: só processa `.zip`, `.rar`, `.7z`, `.gz`, `.txt` (configurável)
- Remove cookies grandes automaticamente para caber em 49MB antes de zipar
- Auto-cleanup do temp dir no startup (previne acumulo após crash)
- Valida grupos a cada 10 ciclos, mata grupos mortos e limpa a fila
- Deduplica por source URL e por `(filename, filesize)`
- Reconnect automático com backoff exponencial (30s → 5min)
- Download paralelo dinâmico: <50MB=4 threads, <200MB=8, ≥200MB=16
- Upload streaming: lê do disco direto pro Supabase sem carregar na RAM
- FLOOD_WAIT: pula grupo no ciclo atual ao invés de retry (evita penalidades maiores)
- Remove pastas de cookies órfãs em batch (log único ao invés de 400+ linhas)
- Suporte a invite links (`https://t.me/+hash`) para grupos privados

## 🏗️ Arquitetura

Clean Architecture: `domain` → `usecase` → `infrastructure`.

```
Telegram MTProto (gotd/td)
     ↓
MonitorGroups — ciclo: scan grupos → enfileira → processa 10 arquivos
     ↓
ProcessFile — download (4-16 threads dinâmicas) → detecta tipo (4 bytes) → extração 7z → parse → upload streaming
     ↓
Supabase Storage
```

Estado persistido em SQLite (WAL). Grupos em `data/groups.json`. Sessão Telegram em `data/session.json`.

## ⚙️ Configuração

```bash
cp .env.example .env
```

```env
TG_API_ID=              # my.telegram.org → API development tools
TG_API_HASH=
TG_PHONE=+55...
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_SERVICE_ROLE_KEY=
SUPABASE_BUCKET=logs
API_KEY=                # autenticação da API REST
```

## 🐳 Deploy

O CI/CD (GitHub Actions) builda o binário Linux, copia configs e sobe o container automaticamente em cada push para `main`.

**Primeira vez no servidor:**

```bash
# Copiar .env pro servidor
scp .env ubuntu@<servidor>:/home/ubuntu/SulphurOG/

# Subir
docker compose up -d --build
docker compose logs -f
```

Na primeira execução o app pede o código de verificação do Telegram no terminal.

**Segredos necessários no GitHub:**

| Secret | Valor |
|--------|-------|
| `SSH_PRIVATE_KEY` | Chave privada para acesso ao servidor |
| `SERVER_HOST` | IP ou hostname do servidor |
| `SERVER_USER` | Usuário SSH (ex: `ubuntu`) |

## 🖥️ Desenvolvimento local

Requer Go 1.21+.

```bash
go run ./cmd/sulphurog/

go vet ./...
go mod tidy
```

## 🌐 API REST

Todas as rotas exigem `X-API-Key: <sua_key>`, exceto `/api/health`.

| Método | Rota | Descrição |
|--------|------|-----------|
| `GET` | `/api/health` | Status do servidor |
| `GET` | `/api/groups` | Lista grupos |
| `POST` | `/api/groups` | Adiciona grupo |
| `GET` | `/api/groups/:id` | Busca grupo |
| `PUT` | `/api/groups/:id` | Atualiza grupo |
| `DELETE` | `/api/groups/:id` | Remove grupo |
| `GET` | `/api/groups/:id/health` | Valida grupo no Telegram |
| `GET` | `/api/status` | Estatísticas gerais |
| `GET` | `/api/stats` | Métricas detalhadas + previsões |

```bash
# Adicionar grupo
curl -X POST http://servidor:8090/api/groups \
  -H "X-API-Key: sua_key" \
  -H "Content-Type: application/json" \
  -d '{"identifier": "https://t.me/NomeDoCanal"}'

# Grupo sem senha (multi-partes)
curl -X POST http://servidor:8090/api/groups \
  -H "X-API-Key: sua_key" \
  -H "Content-Type: application/json" \
  -d '{"identifier": "https://t.me/NomeDoCanal", "ignore_without_password": true}'
```

Aceita username (`https://t.me/canal`) e invite link (`https://t.me/+hash`).

`ignore_without_password: true` faz o monitor não pular arquivos sem senha na mensagem. Útil para canais de multi-partes onde a senha está em outro lugar.

## 📊 Stats endpoint

`GET /api/stats` retorna métricas completas:

```json
{
  "finished": 150,
  "failed": 3,
  "pending": 12,
  "total_bytes": 5368709120,
  "total_ulps": 45200,
  "by_extension": {".zip": 120, ".rar": 25, ".txt": 5},
  "groups_total": 10,
  "groups_active": 8,
  "predictions": {
    "files_per_day": 12.5,
    "files_per_month": 375,
    "avg_file_size_mb": 34.2,
    "bucket_used_gb": 4.8,
    "bucket_free_gb": 45.2,
    "days_until_full": 108
  }
}
```

## ⚙️ Config.yaml

```yaml
processing:
  temp_dir: /tmp/sulphurog
  part_size_kb: 512
  max_retries: 3
  poll_interval: 30s
  threads: 16                     # fallback; dinâmico: <50MB=4, <200MB=8, ≥200MB=16
  process_cookies: true           # false para ignorar cookies
  allowed_extensions:             # extensões aceitas
    - .zip
    - .rar
    - .7z
    - .gz
    - .txt
```

## 📁 Estrutura no Supabase

```
{YYYY-MM-DD_HH-MM-SS}/
├── ulp.txt          ← URL:Login:Senha
├── ulp-2.txt        ← split acima de 49MB
└── cookies.zip
    ├── 1/Chrome_Default.txt
    └── 2/Firefox_Default.txt
```

## 🔒 Variáveis de ambiente

| Variável | Descrição |
|----------|-----------|
| `TG_API_ID` | ID da app Telegram |
| `TG_API_HASH` | Hash da app Telegram |
| `TG_PHONE` | Número com DDI (+55...) |
| `SUPABASE_URL` | URL do projeto Supabase |
| `SUPABASE_SERVICE_ROLE_KEY` | Service role key |
| `SUPABASE_BUCKET` | Nome do bucket |
| `API_KEY` | Chave de autenticação da API |

## 🛠️ Stack

- **Go 1.21+** — binário estático, CGO_ENABLED=0
- **gotd/td** — cliente MTProto nativo
- **Fiber v2** — API REST
- **SQLite** (modernc, pure Go) — tracker de estado com WAL
- **7zip** — extração de ZIP/RAR/7z com senha
- **Supabase Storage** — destino dos logs processados
- **Docker** — runtime no servidor (Oracle Cloud Free Tier, 1GB RAM)
- **GitHub Actions** — CI/CD: build Linux → SCP → docker compose up
