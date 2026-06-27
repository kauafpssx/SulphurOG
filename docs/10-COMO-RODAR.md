# Como Rodar o Projeto

## Instalacao do Go

1. Baixe em: https://go.dev/dl/
2. Instale com as opcoes padrao
3. Abra um **novo** terminal e confirme:
```powershell
go version
```

## Comandos Basicos

```powershell
# Compilar
go build ./cmd/sulphurog/

# Rodar direto
go run ./cmd/sulphurog/

# Rodar com config customizado
go run ./cmd/sulphurog/ configs/config.yaml

# Verificar codigo
go vet ./...

# Limpar modulo
go mod tidy

# Cross-compile pra Linux
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o bin/sulphurog-linux ./cmd/sulphurog/
```

## Tool de Auth Telegram

```powershell
# Configurar .env com TG_API_ID, TG_API_HASH, TG_PHONE
# Rodar (uma unica vez):
go run ./cmd/auth/

# Ele vai:
# 1. Pedir o codigo que chega no Telegram
# 2. Pedir senha 2FA se tiver
# 3. Salvar sessao em data/session.json
```

## Rodar o App

```powershell
# Configurar .env (ver .env.example)
# Rodar:
go run ./cmd/sulphurog/
```

## Endpoints da API

```bash
# Health check
curl -H "X-API-Key: teste" http://localhost:8090/api/health

# Criar grupo
curl -H "X-API-Key: teste" -X POST http://localhost:8090/api/groups \
  -H "Content-Type: application/json" \
  -d '{"identifier":"@DarkSide Hubb","name":"DarkSide Hubb"}'

# Listar grupos
curl -H "X-API-Key: teste" http://localhost:8090/api/groups

# Health do grupo
curl -H "X-API-Key: teste" http://localhost:8090/api/groups/ID/health

# Status geral
curl -H "X-API-Key: teste" http://localhost:8090/api/status
```

## No Ubuntu Server

```bash
# Instalar Go
wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Compilar
go build -ldflags="-s -w" -o bin/sulphurog ./cmd/sulphurog/

# Rodar
./bin/sulphurog configs/config.yaml
```
