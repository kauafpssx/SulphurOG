# Hardware da VM e Otimizacoes

## Especificacoes da VM

| Recurso | Detalhes |
|---------|----------|
| **Shape** | VM.Standard.E2.1.Micro |
| **CPU** | AMD EPYC (1/8 OCPU) |
| **RAM** | 1 GB |
| **Storage** | 50 GB (boot volume) |
| **Rede** | 480 Mbps |
| **Arquitetura** | x86_64 |
| **Burst** | CPU pode usar recursos extras quando disponivel |

## Otimizacoes Implementadas

### 1. Download Sequencial (nao paralelo entre arquivos)

```
❌ ANTES:  3 downloads simultaneos → 3x mais RAM
✅ AGORA:  1 arquivo por vez → ~100MB RAM max
```

- Apenas 1 arquivo e processado por vez
- Download com 16 threads internas (sobre o mesmo arquivo)
- Delay entre arquivos: 5s

### 2. Extracao com 7z (nao extrai tudo pra RAM)

```
❌ ANTES:  Extrair ZIP inteiro na memoria → crash 1GB RAM
✅ AGORA:  7z extrai pra disco, walker le .txt um por um
```

- 7z portable incluido no projeto (`7-ZIP/7z.exe`)
- Extracao pra diretorio temporario no disco
- Walker percorre arquivos `.txt` um por um
- Deleta temporario apos processamento

### 3. Download com 16 Threads + CDN

```
✅ 16 threads paralelas (mesmo arquivo)
✅ CDN do Telegram habilitado
✅ 512KB parts (configuravel via part_size_kb)
✅ Retry handler pra FLOOD_WAIT
```

- Velocidade real: ~2-3 MB/s (media VM)
- Pico: ~20 MB/s (quando nao tem FLOOD_WAIT)
- Velocidade media ignora tempo de FLOOD_WAIT

### 4. Progress Bar Otimizada

```
  ↓ 120MB/335MB [████░░░░░░░░░░░░░░░░░░░░░] 36% 45s 2m30s 2.5MB/s
```

- Velocidade calculada SO durante download ativo
- Ignora tempo de FLOOD_WAIT
- ETA baseado em velocidade ativa
- Atualiza a cada 1 segundo

### 5. Extracao Recursiva (sem estourar RAM)

```
ZIP → extrai pra disco → procura archives dentro → extrai tambem
```

- Se tem RAR dentro de ZIP, extrai tambem
- Copia so `.txt` extraidos pro diretorio pai
- Deleta temporarios apos cada nivel

### 6. Walker Otimizado

```
❌ ANTES:  Percorria TODOS os 4733 arquivos
✅ AGORA:  Percorre todos (necessario pra achar ULPs em qualquer .txt)
```

- Todos os `.txt` sao processados (necessario)
- Mas so le conteudo (nao copia pra RAM)
- Fallback: se parser nao achou, tenta ULP direto

### 7. Memoria por Componente

| Componente | RAM estimada |
|-----------|-------------|
| Go runtime | ~10MB |
| Fiber HTTP | ~5MB |
| gotd/td (conexao) | ~30-50MB |
| 1 download (buffer) | ~10-20MB |
| Extracao 7z (disco) | ~0MB |
| Parse de 1 arquivo | ~1MB |
| **Total medio** | **~60-100MB** |

### 8. Gerenciamento de Disco

```
Download → Extrair → Parse → Upload → Deletar
  500MB     1GB      0MB     0MB      -1.5GB
```

- Deleta ZIP/RAR original apos processamento
- Deleta diretorio temporario apos processamento
- Extracao recursiva limpa apos copiar .txt
- Nunca acumula mais que 1 arquivo temporario

### 9. FLOOD_WAIT Handling

```
Retry handler:
├── Flag (Waiting) = true → progress bar mostra "(Waiting)"
├── Gotd gerencia retry internamente
├── Velocidade media ignora tempo de FLOOD_WAIT
└── Quando passa, volta a baixar normal
```

### 10. Configuracao Ajustavel

```yaml
# config.yaml
processing:
  threads: 16          # threads de download por arquivo
  temp_dir: /tmp/sulphurog
  part_size_kb: 512    # tamanho de cada parte por thread
```

## Comparativo: Antes vs Depois

| Aspecto | Antes | Depois |
|---------|-------|--------|
| Downloads simultaneos | 3 | 1 |
| Extracao | RAM | Disco (7z) |
| Progress bar | So download | Download + extract + parse |
| Velocidade media | Incluia FLOOD_WAIT | Só tempo ativo |
| Extracao recursiva | Nao | Sim |
| Cookies | Mono file | Organizados por vitima |
| Delete original | Nao | Sim |

## Limitacoes da VM

| Recurso | Limite | Status |
|---------|--------|--------|
| RAM | 1GB | OK (~100MB usado) |
| CPU | 1/8 OCPU | OK (burst disponivel) |
| Storage | 50GB | OK (delete imediato) |
| Rede | 480Mbps | OK (~2-3MB/s real) |
