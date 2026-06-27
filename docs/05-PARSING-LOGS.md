# Parsing de Logs de Stealer

## Formatos Suportados

### 1. YAML (Echo Cloud, etc)
```
url: https://example.com/
username: user@email.com
password: mypass123
```

### 2. Key-Value (Outros stealers)
```
Soft: Google Chrome (Default)
Host: https://example.com/
Login: user@email.com
Password: mypass123
-----
Soft: Microsoft Edge (Default)
Host: https://another.com/
Login: admin
Password: pass456
```

### 3. ULP Direto (URL:Login:Pass)
```
https://example.com/user@email.com/mypass123
https://another.com/admin/pass456
```

### 4. Deteccao Automatica
- Se tem `url:` + `password:` → YAML
- Se tem `Host:` + `Password:` → Key-Value
- Se `IsULPFormat` → ULP direto
- Fallback: qualquer linha com 3 partes separadas por `:`

## Estrutura Tipica de um ZIP de Stealer

```
[XX]ID_DATA\
├── passwords.txt           (YAML ou Key-Value)
├── information.txt         (IP, SO, antiviru)
├── Cookies\
│   ├── Chrome_Default.txt  (Netscape format)
│   └── Edge_Default.txt
├── Passwords\
│   ├── Chrome_Default_passwords.txt
│   └── Edge_Default_passwords.txt
├── Soft\
│   ├── Discord/tokens.txt
│   └── Steam/steam_tokens.txt
└── Wallets\
    └── *.json
```

## Headers ASCII

Alguns stealers adicionam cabecalhos:
- `DARKSIDE BRAND`, `REDLINE`, `RACCOON`, `VIDAR`, etc.

O parser faz `StripASCIIHeaders` antes de processar.

## Extracao Recursiva

Se o archive contem outro archive dentro (ZIP dentro de RAR, etc):
1. Extrai o archive principal
2. Procura archives dentro do diretorio extraido
3. Extrai os archives aninhados
4. Copia arquivos `.txt` pro diretorio pai
5. Limpa temporarios

## Deteccao de Arquivos por Magic Bytes

| Tipo | Bytes | Extensao |
|------|-------|----------|
| ZIP | `PK\x03\x04` | .zip |
| RAR | `Rar!` | .rar |
| 7z | `7z\xBC\xAF` | .7z |
| GZ | `\x1F\x8B` | .gz |
| TXT | ASCII puro | .txt |
