# 🔥 SulphurOG

Monitora canais do Telegram, baixa logs de stealer, extrai credenciais e salva no Supabase Storage.

## 🚀 O que faz

- Conecta em canais via MTProto (conta real, sem bot)
- Detecta arquivos novos a cada ciclo, enfileira e processa 10 por vez
- Baixa ZIP/RAR/7z com senha, extrai no disco com 7z, lê `.txt` um por um
- Parseia três formatos de log: ULP simples (`URL:login:senha`), YAML e Key-Value
- Agrupa cookies por vítima, sobe `ulp.txt` + `cookies.zip` pro Supabase
- Divide arquivos ULP acima de 49MB automaticamente
- Valida grupos a cada 10 ciclos, mata grupos mortos e limpa a fila
- Deduplica por source URL e por `(filename, filesize)`
- Reconnect automático com backoff exponencial (30s → 5min)

## 🏗️ Arquitetura

Clean Architecture: `domain` → `usecase` → `infrastructure`.

```
Telegram MTProto (gotd/td)
     ↓
MonitorGroups — ciclo: scan grupos → enfileira → processa 10 arquivos
     ↓
ProcessFile — download → extração 7z → parse → upload
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

```bash
# Adicionar grupo
curl -X POST http://servidor:8090/api/groups \
  -H "X-API-Key: sua_key" \
  -H "Content-Type: application/json" \
  -d '{"identifier": "https://t.me/NomeDoCanal"}'
```

Aceita username (`https://t.me/canal`) e invite link (`https://t.me/+hash`).

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
