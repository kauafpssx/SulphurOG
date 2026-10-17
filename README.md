# SulphurOG

Monitora canais do Telegram, baixa logs de stealer, extrai credenciais e salva localmente em arquivos no host.

## O que faz

- Conecta em canais via MTProto (conta real, sem bot)
- Detecta arquivos novos a cada ciclo, enfileira e processa 10 por vez
- Baixa ZIP/RAR/7z com senha, extrai no disco com 7z, lê `.txt` um por um
- Parseia três formatos de log: ULP simples (`URL:login:senha`), YAML e Key-Value
- Agrupa cookies por vítima, salva `ulp.txt` + `cookies.zip` localmente
- Divide arquivos ULP acima de 49MB automaticamente
- Filtra extensões: só processa `.zip`, `.rar`, `.7z`, `.gz`, `.txt` (configurável)
- Remove cookies grandes automaticamente para caber em 49MB antes de zipar
- Auto-cleanup do temp dir no startup (previne acumulo após crash)
- Valida grupos a cada 10 ciclos, mata grupos mortos e limpa a fila
- Deduplica por source URL e por `(filename, filesize)`
- Reconnect automático com backoff exponencial (30s → 5min)
- Download paralelo dinâmico: <50MB=4 threads, <200MB=8, ≥200MB=16
- FLOOD_WAIT: pula grupo no ciclo atual ao invés de retry (evita penalidades maiores)
- Remove pastas de cookies órfãs em batch (log único ao invés de 400+ linhas)
- Suporte a invite links (`https://t.me/+hash`) para grupos privados

## Arquitetura

Clean Architecture: `domain` → `usecase` → `infrastructure`.

```
Telegram MTProto (gotd/td)
     ↓
MonitorGroups — ciclo: scan grupos → enfileira → processa 10 arquivos
     ↓
ProcessFile — download (4-16 threads dinâmicas) → detecta tipo (4 bytes) → extração 7z → parse → upload local
     ↓
Storage local (data/output)
```

Estado persistido em SQLite (WAL). Grupos em `data/groups.json`. Sessão Telegram em `data/session.json`.

## Configuração

```bash
cp .env.example .env
```

```env
TG_API_ID=              # my.telegram.org → API development tools
TG_API_HASH=
TG_PHONE=+55...
API_KEY=                # autenticação da API REST
```

## Gerenciamento de Grupos

Os grupos são gerenciados **exclusivamente via linha de comando**. As rotas HTTP de criação/atualização/exclusão foram removidas.

### Adicionar grupo

```bash
./sulphurog --addgroup "https://t.me/NomeDoCanal" "Nome Amigável"
```

Aceita username (`https://t.me/canal`), invite link (`https://t.me/+hash`) e ID numérico.

## API REST

Todas as rotas exigem `X-API-Key: <sua_key>`, exceto `/api/health`.

| Método | Rota | Descrição |
|--------|------|-----------|
| `GET` | `/api/health` | Status do servidor |
| `GET` | `/api/groups` | Lista grupos |
| `GET` | `/api/groups/:id` | Busca grupo por ID |
| `GET` | `/api/status` | Estatísticas gerais |
| `GET` | `/api/stats` | Métricas detalhadas |
| `POST` | `/api/monitor/start` | Inicia monitor |
| `POST` | `/api/monitor/pause` | Pausa monitor |
| `GET` | `/api/monitor/status` | Status do monitor |
| `POST` | `/api/master/files` | Super rota para manipulação de arquivos locais |

### Super rota mestre: `/api/master/files`

Recebe requisições POST com os parâmetros do site, tipo de arquivo e opção de remoção.

**Parâmetros:**
- `site` (obrigatório): site para buscar contas
- `file_type`: `txt` ou `cookies` (padrão: `txt`)
- `removeafterget`: `yes` ou `no` (padrão: `no`)

**Exemplo:**
```bash
curl -X POST http://servidor:8090/api/master/files \
  -H "X-API-Key: sua_key" \
  -H "Content-Type: application/json" \
  -d '{
    "site": "exemplo.com",
    "file_type": "txt",
    "removeafterget": "yes"
  }'
```

Quando `removeafterget=yes`, as contas utilizadas são apagadas de todos os arquivos onde forem encontradas.

## Config.yaml

```yaml
api:
  port: 8090
  api_key: ${API_KEY}

telegram:
  api_id: ${TG_API_ID}
  api_hash: ${TG_API_HASH}
  phone: ${TG_PHONE}
  session_file: data/session.json
  channels: []

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

storage:
  output_dir: data/output

logging:
  level: info
```

## Estrutura de armazenamento local

```
data/output/
└── {YYYY-MM-DD_HH-MM-SS}/
    ├── ulp.txt          ← URL:Login:Senha
    ├── ulp-2.txt        ← split acima de 49MB
    └── cookies.zip
        ├── 1/Chrome_Default.txt
        └── 2/Firefox_Default.txt
```

## Variáveis de ambiente

| Variável | Descrição |
|----------|-----------|
| `TG_API_ID` | ID da app Telegram |
| `TG_API_HASH` | Hash da app Telegram |
| `TG_PHONE` | Número com DDI (+55...) |
| `API_KEY` | Chave de autenticação da API |

## Stack

- **Go 1.21+** — binário estático, CGO_ENABLED=0
- **gotd/td** — cliente MTProto nativo
- **Fiber v2** — API REST
- **SQLite** (modernc, pure Go) — tracker de estado com WAL
- **7zip** — extração de ZIP/RAR/7z com senha
- **Docker** — runtime no servidor (Oracle Cloud Free Tier, 1GB RAM)
- **GitHub Actions** — CI/CD: build Linux → SCP → docker compose up