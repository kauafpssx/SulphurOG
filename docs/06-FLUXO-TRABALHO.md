# Fluxo de Trabalho

## Fluxo Principal

```
Monitor (loop infinito)
│
├── 1. Buscar novos arquivos no canal
│   └── Adicionar em Pending (prioridade 1)
│
├── 2. Pegar 1 da fila
│   └── Verificar SHA256 (deduplicacao)
│
├── 3. Download
│   ├── 4 threads + CDN
│   ├── Progress bar: ↓ 120MB/335MB [████░░░░] 36% 45s 2.5MB/s
│   └── Velocidade so conta tempo ativo (ignora FLOOD_WAIT)
│
├── 4. Detectar tipo (magic bytes)
│   ├── text → processULP
│   ├── zip/rar/7z → processStealer
│   └── unknown → tentar como ULP
│
├── 5. ProcessStealer
│   ├── Extrair com 7z (portable)
│   ├── Extracao recursiva (archives aninhados)
│   ├── Walker: todo .txt
│   ├── Parser: YAML → Key-Value → ULP direto → fallback
│   └── Upload: ulp.txt + cookies/N/*.txt
│
├── 6. Cleanup
│   ├── Deletar ZIP/RAR original
│   └── Deletar diretorio temporario
│
├── 7. Tracker
│   └── Marcar COMPLETED (ou FAILED)
│
└── 8. Proximo arquivo
```

## Progress Bar

```
  ↓ 120MB/335MB [████████░░░░░░░░░░░░░░░░░] 36% 45s 2m30s 2.5MB/s
  ↑ atual      total  barra         %   elapsed ETA   velocidade

  (Waiting)  ← aparece quando ta em FLOOD_WAIT

  ✓ 335MB em 120s (2.8 MB/s)  ← download completo
```

## Fases de Processamento

```
downloading...   ← baixando via MTProto
  ↓ 120MB/335MB [████░░░░░░░░░░░░░░░░░░░░░] 36%
✓ 335MB em 120s (2.8 MB/s)
extracting...    ← 7z extraindo
extraction done, scanning files...
nested extraction done, parsing...
scanned=1218 ulps=41439 parsing done
uploading...     ← subindo pro Supabase
done ulps=41439 victims=378 files=1218
```

## Fluxo de Erro

```
Qualquer erro:
├── Deletar arquivo temporario
├── Marcar FAILED no downloaded.json
├── Registrar: erro, timestamp
├── Logar o erro
└── Continuar pro proximo arquivo
```
