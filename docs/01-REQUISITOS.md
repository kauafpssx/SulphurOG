# Requisitos do Projeto

## Requisitos Funcionais

### RF-01: REST API para Grupos Telegram
- CRUD completo de grupos monitorados
- Endpoint de health check por grupo (valida com Telegram real)
- Autenticacao via header `X-API-Key`
- Endpoints:
  - `GET    /api/groups`           — listar grupos
  - `POST   /api/groups`           — adicionar grupo (URL, @username, ou invite link)
  - `GET    /api/groups/:id`       — detalhes
  - `PUT    /api/groups/:id`       — editar (nome, ativo, dead)
  - `DELETE /api/groups/:id`       — remover
  - `GET    /api/groups/:id/health` — status do grupo (ativa Telegram)
  - `GET    /api/status`           — status geral
  - `GET    /api/health`           — health da API

### RF-02: Processamento Sequencial
- **1 arquivo por vez** em toda a pipeline
- Cada etapa roda no seu tempo: download → detect → extract → parse → upload → delete
- Nao ha concorrencia entre arquivos
- Progresso visivel: downloading → extracting → scanning → uploading → done

### RF-03: Download do Telegram
- Conectar via MTProto (gotd/td, conta completa)
- Download com 4 threads paralelas + CDN
- Progress bar com velocidade, ETA, elapsed
- Velocidade media calculada so durante download ativo (ignora FLOOD_WAIT)

### RF-04: Deteccao de Tipo
- Magic bytes: ZIP (PK), RAR (Rar!), 7z (7z), GZ (1F 8B)
- Se ULP: upload direto pro Supabase
- Se stealer: extrair → recursivo → parse → upload
- Se texto com formato ULP: treat como ULP direto

### RF-05: Extracao de Stealer Logs
- Suporta ZIP, RAR, 7z, GZ
- **Extracao recursiva**: se tem outro archive dentro, extrai tambem
- Protegidos por senha (extraida do texto da mensagem)
- Procura `passwords.txt`, `Cookies/*.txt`, e qualquer `.txt` com ULP

### RF-06: Geracao de ULPs
- Formato: `URL:Login:Pass` (um por linha)
- Parser suporta: YAML (url:/username:/password:), Key-Value (Host:/Login:/Password:), e ULP direto
- Fallback: qualquer linha com 3 partes separadas por `:`

### RF-07: Organizacao no Supabase
- ULPs em `ulp.txt` (somente credenciais)
- Cookies organizados por vitima em `cookies/N/arquivo.txt`
- Cada processamento gera uma pasta com timestamp

### RF-08: Tracking (downloaded.json)
- SHA256 do conteudo para deduplicacao
- Status: queued → downloading → processed | failed
- Campos: message_id, file_id, content_hash, source, password, ulp_count, error
- Backup automatico (.bak)
- Atomic write (tmp + rename)

### RF-09: Prioridade por Data
- Novos arquivos tem prioridade (baixar primeiro)
- Depois baixar antigos (recente primeiro)
- Quando terminar lista mapeada, busca mais mensagens

### RF-10: Tratamento de Erros
- **Qualquer erro**: deleta arquivo → mark failed → continua pro proximo
- FLOOD_WAIT: retry com backoff, flag (Waiting) no progress bar
- Senha incorreta: tenta sem senha → senhas comuns → failed
- Arquivo corrompido: failed

### RF-11: Extracao Recursiva
- Se o archive contem outro archive (ZIP dentro de RAR, etc), extrai tambem
- Copia arquivos .txt extraidos pro diretorio pai
- Limpa temporarios apos cada extracao

### RF-12: Exclusao de Arquivos
- ZIP/RAR original deletado apos processamento bem-sucedido
- Diretorio temporario deletado apos processamento
- Em caso de falha: arquivo movido pra failed/ ou deletado

### RF-13: Env vars
- `TG_API_ID`, `TG_API_HASH`, `TG_PHONE` — Telegram
- `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_BUCKET` — Supabase
- `API_KEY`, `API_PORT` — API REST

## Requisitos Nao-Funcionais

### RNF-01: Eficiencia de Memoria
- Operar dentro de 1GB RAM
- Streaming para arquivos grandes (64KB chunks)
- 1 download por vez

### RNF-02: Disponibilidade
- 24/7 como servico systemd
- Reconexao automatica ao Telegram
- Tratamento de FLOOD_WAIT

### RNF-03: Seguranca
- API protegida com X-API-Key
- Credenciais em variaveis de ambiente
- ULPs nunca logados
- 7z portable incluido no projeto
