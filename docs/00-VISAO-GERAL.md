# Visao Geral do Projeto

**Nome:** SulphurOG - Telegram Stealer Log Processor
**Linguagem:** Go
**Alvo:** VM Oracle Cloud Free Tier (1GB RAM, 43GB storage, Ubuntu 24.04)
**Armazenamento:** Supabase Storage (bucket para logs e ULPs)

## O que o projeto faz

Sistema automatizado que:
1. Monitora canais do Telegram via MTProto (conta completa)
2. Baixa arquivos de logs de stealer (ZIP, RAR, 7z)
3. Extrai senha do texto da mensagem do Telegram
4. Extrai credenciais (URL, Login, Pass) dos logs
5. Formata no padrao ULP (URL:Login:Pass)
6. Salva ULPs e cookies no bucket do Supabase
7. Gerencia grupos do Telegram via API REST (CRUD + health)
8. Baixa arquivos por data (novos com prioridade, depois antigos)
9. Mantem JSON de tracking do que ja foi baixado e o que falta
10. Extracao recursiva (se tem outro archive dentro, extrai tambem)

## Arquitetura

```
                    ┌──────────────────┐
                    │   REST API        │
                    │   (Fiber)         │
                    │   /api/groups     │
                    │   /api/health     │
                    │   /api/monitor    │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │   Orchestrator    │
                    │   (Use Cases)     │
                    └──┬─────┬────┬───┘
                       │     │    │
          ┌────────────▼┐ ┌─▼────▼──────────┐
          │  Telegram    │ │  Parser          │
          │  Client      │ │  (stealer logs)  │
          │  (gotd/td)   │ │  ZIP/RAR/7z      │
          └──────┬───────┘ └───────┬──────────┘
                 │                  │
          ┌──────▼───────┐ ┌───────▼──────────┐
          │  Supabase    │ │  Tracker          │
          │  Client      │ │  (JSON state)     │
          │  (REST API)  │ │  downloaded.json  │
          └──────────────┘ └──────────────────┘
```

## Modulos

| Modulo | Pacote | Responsabilidade |
|--------|--------|-----------------|
| REST API | `api/` | CRUD grupos, health check, status, monitor control |
| Telegram Client | `telegram/` | MTProto, downloads, listagem de mensagens |
| Archive Extractor | `extractor/` | ZIP/RAR/7z/GZ com senha, extracao recursiva |
| Log Parser | `parser/` | YAML, Key-Value, ULP direto, deteccao de cookies |
| Supabase Client | `supabase/` | Upload de ULPs e cookies pro bucket |
| Tracker | `tracker/` | downloaded.json, status, deduplicacao por SHA256 |
| Repository | `repository/` | CRUD de grupos em JSON |
| Hash Service | `hash/` | SHA256 de arquivos |

## Fluxo de Processamento

```
1. Monitor busca novos arquivos nos canais do Telegram
2. Para cada arquivo:
   a. Verificar se ja foi baixado (SHA256)
   b. Extrair senha do texto da mensagem
   c. Baixar via MTProto (4 threads, CDN)
   d. Detectar tipo (magic bytes)
   e. Se ULP: upload direto pro Supabase
   f. Se stealer: extrair → recursivo → parse → upload
   g. Deletar arquivo original
   h. Marcar como processed no tracker
```

## Estrutura no Supabase

```
{YYYY-MM-DD_HH-MM-SS}/
├── ulp.txt                     (URL:Login:Pass)
├── cookies/
│   ├── 1/
│   │   ├── Chrome_Default.txt
│   │   └── Edge_Default.txt
│   ├── 2/
│   │   └── Firefox_Profile.txt
│   └── ...
```

## Documentacao

| Arquivo | Conteudo |
|---------|----------|
| [01-REQUISITOS](01-REQUISITOS.md) | Requisitos funcionais e nao-funcionais |
| [02-ARQUITETURA](02-ARQUITETURA.md) | Clean Architecture, interfaces, wire |
| [03-TECNOLOGIAS](03-TECNOLOGIAS.md) | Bibliotecas e ferramentas |
| [04-ESTRUTURA-PROJETO](04-ESTRUTURA-PROJETO.md) | Arvore de pastas |
| [05-PARSING-LOGS](05-PARSING-LOGS.md) | Estrategia de parsing de stealer |
| [06-FLUXO-TRABALHO](06-FLUXO-TRABALHO.md) | Fluxo detalhado com diagramas |
| [07-SEGURANCA](07-SEGURANCA.md) | Seguranca e credenciais |
| [08-IMPLANTACAO](08-IMPLANTACAO.md) | Deploy na VM |
| [09-TAREFAS](09-TAREFAS.md) | Checklist de implementacao |
| [10-COMO-RODAR](10-COMO-RODAR.md) | Como rodar o projeto |
