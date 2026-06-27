# Seguranca

## 1. API REST

- Header `X-API-Key` obrigatorio em todos os endpoints
- Chave em variavel de ambiente `API_KEY`
- Retornar `401 Unauthorized` se ausente ou invalida

## 2. Credenciais do Telegram

- Variaveis de ambiente: `TG_API_ID`, `TG_API_HASH`, `TG_PHONE`
- Sessao em `data/session.json` com permissoes `600`
- Conta dedicada (nao pessoal)
- Se comprometida: revogar via Telegram Settings > Devices

## 3. Supabase

- `SUPABASE_SERVICE_ROLE_KEY` para uploads (bypassa RLS)
- Bucket privado (nao publico)
- Chaves em variaveis de ambiente

## 4. Dados ULP

- ULPs **nunca** logados
- Arquivos temporarios deletados apos upload
- Logs da aplicacao nao devem conter credenciais

## 5. Permissoes da VM

```bash
chmod 750 /opt/sulphurog/
chmod 600 /opt/sulphurog/.env
chmod 600 /opt/sulphurog/data/session.json
```

## 6. Firewall

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw enable
```
