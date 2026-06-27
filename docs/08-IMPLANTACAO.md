# Implantacao e Deploy

## Setup da VM

```bash
# Atualizar
sudo apt update && sudo apt upgrade -y

# Dependencias
sudo apt install -y p7zip-full unzip curl git

# Swap (2GB)
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab

# Usuario
sudo useradd -r -s /bin/false -d /opt/sulphurog sulphurog
sudo mkdir -p /opt/sulphurog/{data,output,logs,failed,data/temp}
sudo chown -R sulphurog:sulphurog /opt/sulphurog
```

## Deploy

```bash
# Compilar (Windows)
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o bin/sulphurog.exe ./cmd/sulphurog/

# Transferir
scp bin/sulphurog.exe user@vm:/opt/sulphurog/sulphurog
scp configs/config.yaml user@vm:/opt/sulphurog/config.yaml
scp 7-ZIP/7z.exe user@vm:/opt/sulphurog/7-ZIP/7z.exe
```

## Systemd

```ini
[Unit]
Description=SulphurOG Telegram Log Processor
After=network.target

[Service]
Type=simple
User=sulphurog
WorkingDirectory=/opt/sulphurog
ExecStart=/opt/sulphurog/sulphurog
Restart=always
RestartSec=10
MemoryMax=512M

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable sulphurog
sudo systemctl start sulphurog
```

## Monitoramento

```bash
sudo journalctl -u sulphurog -f
curl -H "X-API-Key: teste" http://localhost:8080/api/health
curl -H "X-API-Key: teste" http://localhost:8080/api/status
```
