# Checklist de Implementacao

## Fase 1: Setup ✅
- [x] go mod init, Makefile, .gitignore, config.yaml

## Fase 2: Domain Layer ✅
- [x] group.go, log_file.go, ulp.go, extracted_data.go, tracker.go, interfaces.go

## Fase 3: API REST ✅
- [x] Fiber server, X-API-Key, CRUD, health, status, monitor

## Fase 4: Repository ✅
- [x] JSON group repo com atomic write, thread-safe

## Fase 5: Telegram ✅
- [x] MTProto (gotd/td), downloads com 4 threads + CDN
- [x] Progress bar com velocidade, ETA, FLOOD_WAIT
- [x] ResolveChannel (URL, @username, invite link)
- [x] ListFiles, GetMessages, GetChannelStatus

## Fase 6: Extractor ✅
- [x] Magic bytes (ZIP, RAR, 7z, GZ)
- [x] ZIP com senha (alexmullins/zip)
- [x] RAR/7z via 7z CLI (portable incluido)
- [x] Extracao recursiva (archives aninhados)

## Fase 7: Parser ✅
- [x] YAML (url:/username:/password:)
- [x] Key-Value (Host:/Login:/Password:)
- [x] ULP direto (URL:Login:Pass)
- [x] Fallback generico
- [x] Deteccao de cookies (Netscape format)

## Fase 8: Supabase ✅
- [x] REST client para Storage API
- [x] Upload de bytes

## Fase 9: Tracker ✅
- [x] downloaded.json com SHA256
- [x] Status: queued→downloading→processed|failed
- [x] Backup automatico (.bak)
- [x] Atomic write

## Fase 10: Monitor ✅
- [x] Loop sequencial 1 arquivo por vez
- [x] Prioridade por data (novos primeiro)
- [x] Busca continua quando lista esgota

## Fase 11: Deploy
- [ ] Cross-compile linux/amd64
- [ ] deploy.sh + setup-vm.sh
- [ ] systemd service
