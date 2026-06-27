# 🔥 SulphurOG

Monitor automático de canais do Telegram que baixa logs de stealer, extrai credenciais e salva no Supabase.

## 🚀 O que faz

- Monitora canais do Telegram via MTProto (conta real, sem bot)
- Baixa arquivos ZIP/RAR/7z e extrai com senha automática
- Parseia logs de stealer → formato `URL:Login:Senha`
- Faz split de arquivos ULP acima de 49MB automaticamente
- Sobe tudo pro Supabase Storage
- Valida periodicamente se os grupos ainda estão vivos
- Detecta canal deletado durante download e limpa a fila
- Reconnect automático se o Telegram cair

## 🏗️ Arquitetura

Clean Architecture em Go — domain → usecase → infrastructure.

```
Telegram MTProto
     ↓
Monitor (loop sequencial)
     ↓
ProcessFile (download → extrai → parseia → upload)
     ↓
Supabase Storage
```

Tracker em SQLite com WAL. Grupos em `data/groups.json`. Sessão Telegram em `data/session.json`.

## ⚙️ Configuração

Copia o `.env.example`:

```bash
cp .env.example .env
```

Preenche:

```env
TG_API_ID=           # my.telegram.org → API development tools
TG_API_HASH=
TG_PHONE=+55...
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_SERVICE_ROLE_KEY=
SUPABASE_BUCKET=logs
API_KEY=             # chave pra API REST
```

## 🐳 Deploy (Docker)

```bash
# Primeira vez
cp .env.example .env
nano .env

docker compose up -d --build
docker compose logs -f
```

Na primeira execução o app vai pedir o código de verificação do Telegram no terminal.

### Atualizar após mudança no código

```bash
# Windows — builda o binário pra Linux
build.bat

# Sobe o binário pro servidor e reinicia
docker compose restart
```

## 🖥️ Desenvolvimento local

Requer Go 1.26+.

```bash
# Rodar
go run ./cmd/sulphurog/

# Build Linux
build.bat

# Verificar
go vet ./...
go mod tidy
```

## 🌐 API REST

Todas as rotas exigem `X-API-Key: <sua_key>` no header, exceto `/api/health`.

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

### Adicionar grupo

```bash
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
├── ulp-2.txt        ← split automático acima de 49MB
└── cookies/
    ├── 1/
    │   └── Chrome_Default.txt
    └── 2/
        └── Firefox_Default.txt
```

## 🔒 Variáveis de ambiente

| Variável | Descrição |
|----------|-----------|
| `TG_API_ID` | ID da app Telegram (my.telegram.org) |
| `TG_API_HASH` | Hash da app Telegram |
| `TG_PHONE` | Número com DDI (+55...) |
| `SUPABASE_URL` | URL do projeto Supabase |
| `SUPABASE_SERVICE_ROLE_KEY` | Service role key do Supabase |
| `SUPABASE_BUCKET` | Nome do bucket |
| `API_KEY` | Chave de autenticação da API REST |

## 🛠️ Stack

- **Go 1.26** — binário estático, sem CGO
- **gotd/td** — cliente Telegram MTProto nativo
- **Fiber v2** — API REST
- **SQLite** (modernc, pure Go) — tracker de estado
- **7zip** — extração de RAR/7z/ZIP com senha
- **Supabase Storage** — destino dos logs processados
- **Docker** — deploy na Oracle Cloud Free Tier
