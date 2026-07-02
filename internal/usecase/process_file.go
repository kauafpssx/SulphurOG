package usecase

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/sulphurog/sulphurog/internal/domain"
	"github.com/sulphurog/sulphurog/internal/infrastructure/extractor"
)

type ProcessFileUseCase struct {
	telegram       domain.TelegramClient
	extractor      domain.ArchiveExtractor
	parser         domain.LogParser
	storage        domain.SupabaseStorage
	tracker        domain.Tracker
	hasher         domain.HashService
	tempDir        string
	bucket         string
	processCookies bool
	log            zerolog.Logger
}

func NewProcessFileUseCase(
	telegram domain.TelegramClient,
	extractor domain.ArchiveExtractor,
	parser domain.LogParser,
	storage domain.SupabaseStorage,
	tracker domain.Tracker,
	hasher domain.HashService,
	tempDir string,
	bucket string,
	processCookies bool,
	log zerolog.Logger,
) *ProcessFileUseCase {
	return &ProcessFileUseCase{
		telegram:       telegram,
		extractor:      extractor,
		parser:         parser,
		storage:        storage,
		tracker:        tracker,
		hasher:         hasher,
		tempDir:        tempDir,
		bucket:         bucket,
		processCookies: processCookies,
		log:            log,
	}
}

const maxUploadSize = 50 * 1024 * 1024   // 50MB — limite Supabase
const splitULPThreshold = 49 * 1024 * 1024 // 49MB por parte — margem de segurança

func (uc *ProcessFileUseCase) Execute(ctx context.Context, file domain.LogFile) (retErr error) {
	log := uc.log.With().Str("file", file.Filename).Logger()

	already, _ := uc.tracker.IsDownloaded(file.ContentHash)
	if already {
		return nil
	}

	// Skip split RAR antes de baixar (economiza banda)
	if splitRarPattern.MatchString(file.Filename) {
		return uc.failDownload(file.ContentHash, fmt.Errorf("split RAR archive — needs all parts, skipping"))
	}

	record := domain.FileRecord{
		MessageID:   file.MessageID,
		FileID:      file.FileID,
		ContentHash: file.ContentHash,
		Source:      file.SourceURL,
		Filename:    file.Filename,
		FileSize:    file.FileSize,
		Password:    file.Password,
	}
	uc.tracker.MarkDownloaded(record)

	destPath := filepath.Join(uc.tempDir, file.Filename)
	os.MkdirAll(uc.tempDir, 0755)

	// Deleta archive em caso de erro, EXCETO bucket cheio (arquivo fica pra retry)
	defer func() {
		if retErr != nil && !errors.Is(retErr, domain.ErrStorageFull) {
			os.Remove(destPath)
		}
	}()

	if file.FileLocation != nil {
		// Threads dinâmicas: arquivo pequeno = poucas threads, grande = mais
		threads := uc.optimalThreads(file.FileSize)
		bytes, err := uc.telegram.DownloadFile(ctx, file.FileLocation, destPath, file.FileSize, threads)
		if err != nil {
			return uc.failDownload(file.ContentHash, fmt.Errorf("download: %w", err))
		}
		log.Info().Int64("bytes", bytes).Int("threads", threads).Msg("downloaded")
		uc.tracker.MarkDownloadComplete(file.ContentHash)
	} else {
		return uc.failDownload(file.ContentHash, fmt.Errorf("no file location"))
	}

	// Detecta tipo lendo só 4 bytes do disco (não carrega o arquivo inteiro)
	fileType := uc.detectFileType(destPath)
	log.Info().Str("type", fileType).Msg("detected")

	switch fileType {
	case "text":
		// Texto: lê na RAM pra parsear
		content, err := os.ReadFile(destPath)
		if err != nil {
			return uc.failDownload(file.ContentHash, fmt.Errorf("read: %w", err))
		}
		return uc.processULP(ctx, file, content)
	case "zip", "rar", "7z", "gz":
		// Archive: extrai do disco direto, sem ler na RAM
		return uc.processStealer(ctx, file, destPath)
	default:
		// Desconhecido: lê só os primeiros 4KB pra checar se é ULP
		header := make([]byte, 4096)
		f, err := os.Open(destPath)
		if err != nil {
			return uc.failDownload(file.ContentHash, fmt.Errorf("open: %w", err))
		}
		n, _ := f.Read(header)
		f.Close()
		if isULPContent(header[:n]) {
			content, err := os.ReadFile(destPath)
			if err != nil {
				return uc.failDownload(file.ContentHash, fmt.Errorf("read: %w", err))
			}
			return uc.processULP(ctx, file, content)
		}
		return uc.failDownload(file.ContentHash, fmt.Errorf("unsupported: %s", fileType))
	}
}

func (uc *ProcessFileUseCase) processULP(ctx context.Context, file domain.LogFile, content []byte) error {
	log := uc.log.With().Str("file", file.Filename).Logger()

	deduped := deduplicateLines(string(content))
	ulpCount := strings.Count(deduped, "\n")

	folderName := time.Now().Format("2006-01-02_15-04-05")

	uc.tracker.MarkUploading(file.ContentHash)
	firstPath, parts, err := uc.uploadULPContent(ctx, folderName, deduped)
	if err != nil {
		if errors.Is(err, domain.ErrStorageFull) {
			uc.tracker.MarkDownloadComplete(file.ContentHash)
			return err
		}
		return uc.failDownload(file.ContentHash, fmt.Errorf("upload: %w", err))
	}

	uc.tracker.MarkFinished(file.ContentHash, firstPath, ulpCount)
	os.Remove(filepath.Join(uc.tempDir, file.Filename))
	log.Info().Int("ulps", ulpCount).Int("parts", parts).Msg("ULP uploaded")
	return nil
}

var splitRarPattern = regexp.MustCompile(`(?i)\.part\d+\.rar$`)

func (uc *ProcessFileUseCase) processStealer(ctx context.Context, file domain.LogFile, archivePath string) error {
	log := uc.log.With().Str("file", file.Filename).Logger()

	// Fix D: detectar split RAR (part01.rar, part02.rar, etc.) e pular
	if splitRarPattern.MatchString(file.Filename) {
		return uc.failDownload(file.ContentHash, fmt.Errorf("split RAR archive — needs all parts, skipping"))
	}

	// Fix E: validar magic bytes antes de extrair
	fileType := uc.detectFileType(archivePath)
	if fileType != "zip" && fileType != "rar" && fileType != "7z" && fileType != "gz" {
		return uc.failDownload(file.ContentHash, fmt.Errorf("invalid archive (magic bytes: %s)", fileType))
	}

	ts := time.Now().Format("150405")
	extractDir := filepath.Join(uc.tempDir, "ext_"+ts)
	stagingDir := filepath.Join(uc.tempDir, "stage_"+ts)
	os.MkdirAll(extractDir, 0755)
	os.MkdirAll(stagingDir, 0755)
	defer os.RemoveAll(extractDir)
	defer os.RemoveAll(stagingDir)

	log.Info().Msg("extracting...")
	if err := extractor.ExtractToTemp(archivePath, file.Password, extractDir); err != nil {
		return uc.failDownload(file.ContentHash, fmt.Errorf("7z failed: %w", err))
	}
	log.Info().Msg("extraction done, scanning files...")

	uc.extractNested(extractDir, file.Password)
	log.Info().Msg("nested extraction done, parsing...")

	var allULPs []domain.ULP
	filesScanned := 0
	victimIdx := 0
	cookiesStagingDir := filepath.Join(stagingDir, "cookies")

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		// Coletar cookies e pular descida na pasta
		if info.IsDir() && strings.ToLower(info.Name()) == "cookies" {
			victimIdx++
			destDir := filepath.Join(cookiesStagingDir, fmt.Sprintf("%d", victimIdx))
			os.MkdirAll(destDir, 0755)
			entries, _ := os.ReadDir(path)
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
					continue
				}
				data, readErr := os.ReadFile(filepath.Join(path, entry.Name()))
				if readErr != nil {
					continue
				}
				os.WriteFile(filepath.Join(destDir, entry.Name()), data, 0644)
			}
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		filesScanned++
		text := string(content)

		ulps := uc.parser.ParsePasswords(text)
		allULPs = append(allULPs, ulps...)
		return nil
	})

	log.Info().Int("scanned", filesScanned).Int("ulps", len(allULPs)).Int("victims", victimIdx).Msg("parsing done")

	if len(allULPs) == 0 {
		return uc.failDownload(file.ContentHash, fmt.Errorf("no ULPs found"))
	}

	// Montar ulp.txt no staging (deduplicado)
	ulpStagingPath := filepath.Join(stagingDir, "ulp.txt")
	seen := make(map[string]struct{}, len(allULPs))
	var ulpContent strings.Builder
	for _, ulp := range allULPs {
		line := ulp.String()
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		ulpContent.WriteString(line + "\n")
	}
	if err := os.WriteFile(ulpStagingPath, []byte(ulpContent.String()), 0644); err != nil {
		return uc.failDownload(file.ContentHash, fmt.Errorf("write ulp staging: %w", err))
	}

	// Zipar cookies no staging
	var hasCookiesZip bool
	cookiesZipPath := filepath.Join(stagingDir, "cookies.zip")
	if victimIdx > 0 && uc.processCookies {
		// Calcula tamanho de cada pasta de vítima
		type victimDir struct {
			path string
			size int64
		}
		var victims []victimDir
		var totalSize int64
		for i := 1; i <= victimIdx; i++ {
			dir := filepath.Join(cookiesStagingDir, fmt.Sprintf("%d", i))
			s := dirSize(dir)
			victims = append(victims, victimDir{path: dir, size: s})
			totalSize += s
		}

		// Remove pastas maiores até caber em 49MB
		const maxSize = 49 * 1024 * 1024
		if totalSize > maxSize {
			// Ordena por tamanho desc (maiores primeiro)
			for i := 0; i < len(victims)-1; i++ {
				for j := i + 1; j < len(victims); j++ {
					if victims[j].size > victims[i].size {
						victims[i], victims[j] = victims[j], victims[i]
					}
				}
			}
		var removedCount int
		var freedMB int64
		for _, v := range victims {
			if totalSize <= maxSize {
				break
			}
			os.RemoveAll(v.path)
			totalSize -= v.size
			freedMB += v.size / 1024 / 1024
			removedCount++
		}
		if removedCount > 0 {
			log.Info().Int("removed", removedCount).Int64("freed_mb", freedMB).Msg("trimmed cookie folders to fit 49MB")
		}
		}

		if err := zipDir(cookiesStagingDir, cookiesZipPath); err != nil {
			log.Warn().Err(err).Msg("failed to zip cookies, skipping")
		} else {
			hasCookiesZip = true
		}
		os.RemoveAll(cookiesStagingDir)
	} else if victimIdx > 0 {
		// Cookies desativados, limpa direto
		os.RemoveAll(cookiesStagingDir)
	}

	folderName := time.Now().Format("2006-01-02_15-04-05")
	log.Info().Msg("uploading...")
	uc.tracker.MarkUploading(file.ContentHash)

	// ULP — split automatico se > 49MB
	ulpData, _ := os.ReadFile(ulpStagingPath)
	firstPath, ulpParts, err := uc.uploadULPContent(ctx, folderName, string(ulpData))
	if err != nil {
		if errors.Is(err, domain.ErrStorageFull) {
			uc.tracker.MarkDownloadComplete(file.ContentHash)
			return err
		}
		return uc.failDownload(file.ContentHash, fmt.Errorf("upload ulp: %w", err))
	}
	if ulpParts > 1 {
		log.Info().Int("parts", ulpParts).Msg("ULP split")
	}

	// Cookies — binario, nao splita
	if hasCookiesZip {
		zipData, _ := os.ReadFile(cookiesZipPath)
		if len(zipData) > maxUploadSize {
			log.Warn().Int("size_mb", len(zipData)/1024/1024).Msg("cookies.zip >50MB, skipping")
		} else if len(zipData) > 0 {
			cookiesRemotePath := fmt.Sprintf("%s/cookies.zip", folderName)
			if err := uc.storage.Upload(ctx, uc.bucket, cookiesRemotePath, zipData); err != nil {
				log.Warn().Err(err).Msg("failed to upload cookies.zip")
			} else {
				log.Info().Str("path", cookiesRemotePath).Msg("cookies uploaded")
			}
		}
	}

	uc.tracker.MarkFinished(file.ContentHash, firstPath, len(allULPs))
	os.Remove(archivePath)
	log.Info().Int("ulps", len(allULPs)).Int("victims", victimIdx).Int("files", filesScanned).Msg("done")
	return nil
}

func (uc *ProcessFileUseCase) extractNested(dir, password string) {
	// Coleta archives primeiro, depois extrai — evita modificar dir durante Walk
	var archives []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		lower := strings.ToLower(info.Name())
		if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".rar") ||
			strings.HasSuffix(lower, ".7z") || strings.HasSuffix(lower, ".gz") {
			archives = append(archives, path)
		}
		return nil
	})

	for _, path := range archives {
		nestedDir := path + "_extracted"
		os.MkdirAll(nestedDir, 0755)

		if err := extractor.ExtractToTemp(path, password, nestedDir); err == nil {
			filepath.Walk(nestedDir, func(nestedPath string, nestedInfo os.FileInfo, err error) error {
				if err != nil || nestedInfo == nil || nestedInfo.IsDir() {
					return nil
				}
				if strings.HasSuffix(strings.ToLower(nestedInfo.Name()), ".txt") {
					content, readErr := os.ReadFile(nestedPath)
					if readErr != nil {
						return nil
					}
					destPath := filepath.Join(dir, nestedInfo.Name())
					if _, err := os.Stat(destPath); err == nil {
						destPath = filepath.Join(dir, nestedInfo.Name()+"_nested")
					}
					os.WriteFile(destPath, content, 0644)
				}
				return nil
			})
		}
		os.RemoveAll(nestedDir)
	}
}

func (uc *ProcessFileUseCase) failDownload(contentHash string, err error) error {
	uc.log.Error().Err(err).Str("hash", contentHash).Msg("failed")
	uc.tracker.MarkFailed(contentHash, err.Error())
	return err
}

func (uc *ProcessFileUseCase) detectFileType(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return "unknown"
	}

	if header[0] == 'P' && header[1] == 'K' && header[2] == 0x03 && header[3] == 0x04 {
		return "zip"
	}
	if header[0] == 'R' && header[1] == 'a' && header[2] == 'r' && header[3] == '!' {
		return "rar"
	}
	if header[0] == '7' && header[1] == 'z' && header[2] == 0xBC && header[3] == 0xAF {
		return "7z"
	}
	if header[0] == 0x1F && header[1] == 0x8B {
		return "gz"
	}
	for _, b := range header {
		if b == 0 {
			return "unknown"
		}
	}
	return "text"
}

func deduplicateLines(content string) string {
	seen := make(map[string]struct{})
	var b strings.Builder
	start := 0
	for i := 0; i <= len(content); i++ {
		if i == len(content) || content[i] == '\n' {
			line := strings.TrimSpace(content[start:i])
			start = i + 1
			if line == "" {
				continue
			}
			if _, dup := seen[line]; !dup {
				seen[line] = struct{}{}
				b.WriteString(line + "\n")
			}
		}
	}
	return b.String()
}

// splitULPContent divide em partes de ate splitULPThreshold bytes, quebrando em linha.
func splitULPContent(content string) []string {
	if len(content) <= splitULPThreshold {
		return []string{content}
	}
	var parts []string
	var cur strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			continue
		}
		lineBytes := len(line) + 1
		if cur.Len() > 0 && cur.Len()+lineBytes > splitULPThreshold {
			parts = append(parts, cur.String())
			cur.Reset()
		}
		cur.WriteString(line + "\n")
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// optimalThreads retorna número ideal de threads baseado no tamanho do arquivo.
// Arquivos pequenos usam menos threads (menos overhead).
func (uc *ProcessFileUseCase) optimalThreads(fileSize int64) int {
	switch {
	case fileSize < 50*1024*1024: // <50MB
		return 4
	case fileSize < 200*1024*1024: // <200MB
		return 8
	default: // >=200MB
		return 16
	}
}

// uploadULPContent faz split automatico se > 49MB e faz upload de cada parte.
// Retorna path da primeira parte e numero de partes.
func (uc *ProcessFileUseCase) uploadULPContent(ctx context.Context, folderName, content string) (firstPath string, parts int, err error) {
	chunks := splitULPContent(content)
	parts = len(chunks)
	for i, chunk := range chunks {
		var remotePath string
		if parts == 1 {
			remotePath = fmt.Sprintf("%s/ulp.txt", folderName)
		} else {
			remotePath = fmt.Sprintf("%s/ulp-%d.txt", folderName, i+1)
		}
		if i == 0 {
			firstPath = remotePath
		}
		if err = uc.storage.Upload(ctx, uc.bucket, remotePath, []byte(chunk)); err != nil {
			return firstPath, parts, err
		}
	}
	return firstPath, parts, nil
}

func isULPContent(content []byte) bool {
	lines := strings.SplitN(string(content), "\n", 10)
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "\t") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		// Reconstrói URL (todos os parts exceto últimos 2 que são login:pass)
		url := strings.ToLower(strings.Join(parts[:len(parts)-2], ":"))
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			count++
		}
	}
	return count >= 3
}

func zipDir(sourceDir, destZip string) error {
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(sourceDir, path)
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		f, err := w.Create(relPath)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(f, src)
		return err
	})
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}
