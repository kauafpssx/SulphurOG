package telegram

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
	"github.com/rs/zerolog"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type GotdClient struct {
	apiID         int
	apiHash       string
	phone         string
	sessionFile   string
	client        *telegram.Client
	api           *tg.Client
	dl            *downloader.Downloader
	log           zerolog.Logger
	threads       int
	waiting       bool
	mu            sync.RWMutex
	ready         chan struct{}
	fileLocations map[string]*tg.InputDocumentFileLocation
	fileLocMu     sync.RWMutex
}

func NewGotdClient(apiID int, apiHash, phone, sessionFile string, threads, partSizeKB int, log zerolog.Logger) *GotdClient {
	if threads <= 0 {
		threads = 16
	}
	if partSizeKB <= 0 {
		partSizeKB = 512
	}

	c := &GotdClient{
		apiID:         apiID,
		apiHash:       apiHash,
		phone:         phone,
		sessionFile:   sessionFile,
		log:           log,
		threads:       threads,
		ready:         make(chan struct{}),
		fileLocations: make(map[string]*tg.InputDocumentFileLocation),
	}

	c.dl = downloader.NewDownloader().
		WithPartSize(partSizeKB * 1024).
		WithAllowCDN(true).
		WithRetryHandler(func(e downloader.RetryEvent) {
			c.mu.Lock()
			c.waiting = isFloodWait(e.Err)
			c.mu.Unlock()
		})

	return c
}

func (c *GotdClient) waitForReady(ctx context.Context) error {
	// Captura o canal atual sob lock pra suportar reconnect
	c.mu.RLock()
	ready := c.ready
	c.mu.RUnlock()

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for telegram")
	}
}

// Connect estabelece conexão com Telegram. Pode ser chamado novamente após desconexão.
func (c *GotdClient) Connect(ctx context.Context) error {
	// Novo canal ready para cada tentativa de conexão
	ready := make(chan struct{})
	c.mu.Lock()
	c.ready = ready
	c.mu.Unlock()

	// Limpar cache de locations na reconexão
	c.fileLocMu.Lock()
	c.fileLocations = make(map[string]*tg.InputDocumentFileLocation)
	c.fileLocMu.Unlock()

	storage := &session.FileStorage{Path: c.sessionFile}
	client := telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: storage,
	})

	return client.Run(ctx, func(ctx context.Context) error {
		c.mu.Lock()
		c.client = client
		c.api = client.API()
		c.mu.Unlock()

		close(ready) // seguro: cada Run tem seu próprio ready

		status, err := client.Auth().Status(ctx)
		if err == nil && status.Authorized {
			c.log.Info().Msg("telegram: connected")
			<-ctx.Done()
			return ctx.Err()
		}

		c.log.Info().Msg("telegram: authenticating...")
		flow := auth.NewFlow(&terminalAuth{phone: c.phone}, auth.SendCodeOptions{})
		if err := flow.Run(ctx, client.Auth()); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
		c.log.Info().Msg("telegram: authenticated")

		<-ctx.Done()
		return ctx.Err()
	})
}

func (c *GotdClient) ResolveChannel(identifier string) (bool, int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.waitForReady(ctx); err != nil {
		return false, 0, 0, err
	}

	c.mu.RLock()
	api := c.api
	c.mu.RUnlock()

	if strings.Contains(identifier, "t.me/") {
		parts := strings.Split(identifier, "t.me/+")
		if len(parts) > 1 {
			hash := strings.TrimSuffix(parts[1], "/")
			return c.resolveInvite(api, hash)
		}
	}

	username := identifier
	if strings.Contains(identifier, "t.me/") {
		parts := strings.Split(identifier, "t.me/")
		if len(parts) > 1 {
			username = strings.TrimSuffix(parts[1], "/")
			username = strings.TrimPrefix(username, "+")
		}
	}
	username = strings.TrimPrefix(username, "@")

	resp, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return false, 0, 0, err
	}

	for _, chat := range resp.Chats {
		if ch, ok := chat.(*tg.Channel); ok {
			return true, ch.ID, ch.AccessHash, nil
		}
	}

	return false, 0, 0, fmt.Errorf("channel not found: %s", identifier)
}

func (c *GotdClient) resolveInvite(api *tg.Client, hash string) (bool, int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	checkResp, err := api.MessagesCheckChatInvite(ctx, hash)
	if err == nil {
		if r, ok := checkResp.(*tg.ChatInviteAlready); ok {
			if ch, ok := r.Chat.(*tg.Channel); ok {
				return true, ch.ID, ch.AccessHash, nil
			}
		}
	}

	_, err = api.MessagesImportChatInvite(ctx, hash)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "USER_ALREADY_PARTICIPANT") {
			return false, 0, 0, fmt.Errorf("already in channel, use @username")
		}
		return false, 0, 0, err
	}

	return true, 0, 0, nil
}

func (c *GotdClient) GetMessages(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]domain.LogFile, error) {
	if err := c.waitForReady(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	api := c.api
	c.mu.RUnlock()

	peer := &tg.InputPeerChannel{ChannelID: channelID, AccessHash: accessHash}
	var result []domain.LogFile

	iterator := query.Messages(api).GetHistory(peer)
	if err := iterator.ForEach(ctx, func(ctx context.Context, elem messages.Elem) error {
		if len(result) >= limit {
			return fmt.Errorf("limit reached")
		}
		msg, ok := elem.Msg.(*tg.Message)
		if !ok || msg.Media == nil || beforeID > 0 && msg.ID >= beforeID {
			return nil
		}
		doc, ok := msg.Media.(*tg.MessageMediaDocument)
		if !ok {
			return nil
		}
		d, ok := doc.Document.AsNotEmpty()
		if !ok {
			return nil
		}
		filename := "unknown"
		for _, attr := range d.Attributes {
			if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
				filename = fn.FileName
				break
			}
		}
		password := extractPasswordFromMessage(msg.Message)
		fileLoc := d.AsInputDocumentFileLocation("")
		cacheKey := fmt.Sprintf("%d_%d", channelID, msg.ID)
		c.fileLocMu.Lock()
		if len(c.fileLocations) > 500 {
			c.fileLocations = make(map[string]*tg.InputDocumentFileLocation)
		}
		c.fileLocations[cacheKey] = fileLoc
		c.fileLocMu.Unlock()
		result = append(result, domain.LogFile{
			ID:           fmt.Sprintf("%d_%d", channelID, msg.ID),
			MessageID:    msg.ID,
			FileID:       fmt.Sprintf("%d", d.ID),
			SourceURL:    fmt.Sprintf("https://t.me/c/%d/%d", channelID, msg.ID),
			Filename:     filename,
			FileSize:     d.Size,
			Date:         domain.UnixToDate(int64(msg.Date)),
			Password:     password,
			ContentHash:  fmt.Sprintf("%d_%d_%s", channelID, msg.ID, filename),
			FileLocation: fileLoc,
		})
		return nil
	}); err != nil && err.Error() != "limit reached" {
		return nil, err
	}

	return result, nil
}

func (c *GotdClient) ListFiles(ctx context.Context, channelID int64, accessHash int64, limit int, beforeID int) ([]domain.LogFile, error) {
	return c.GetMessages(ctx, channelID, accessHash, limit, beforeID)
}

func (c *GotdClient) DownloadFile(ctx context.Context, location interface{}, destPath string, totalSize int64, threads int) (int64, error) {
	if err := c.waitForReady(ctx); err != nil {
		return 0, err
	}

	c.mu.RLock()
	api := c.api
	c.mu.RUnlock()

	loc, ok := location.(*tg.InputDocumentFileLocation)
	if !ok {
		return 0, fmt.Errorf("invalid location: %T", location)
	}

	if threads <= 0 {
		threads = c.threads
	}

	startTime := time.Now()
	done := make(chan struct{})
	go c.monitorProgress(destPath, totalSize, startTime, done)
	defer close(done)

	_, err := c.dl.Download(api, loc).WithThreads(threads).ToPath(ctx, destPath)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return 0, fmt.Errorf("stat after download: %w", err)
	}

	elapsed := time.Since(startTime).Seconds()
	speed := float64(info.Size()) / elapsed / 1024 / 1024
	fmt.Printf("\r  ✓ %s em %.0fs (%.1f MB/s)          \n", formatSize(info.Size()), elapsed, speed)
	return info.Size(), nil
}

func (c *GotdClient) monitorProgress(destPath string, totalSize int64, startTime time.Time, done <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var activeTime time.Duration
	var lastCheck = time.Now()
	var lastSizeCheck int64

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := os.Stat(destPath)
			if err != nil {
				continue
			}
			currentSize := info.Size()
			now := time.Now()

			c.mu.RLock()
			waiting := c.waiting
			c.mu.RUnlock()

			if !waiting && currentSize > lastSizeCheck {
				activeTime += now.Sub(lastCheck)
			}
			lastSizeCheck = currentSize
			lastCheck = now

			speed := 0.0
			if activeTime.Seconds() > 0 {
				speed = float64(currentSize) / activeTime.Seconds() / 1024 / 1024
			}

			elapsed := now.Sub(startTime).Seconds()
			eta := 0.0
			if speed > 0 && totalSize > currentSize {
				eta = float64(totalSize-currentSize) / (speed * 1024 * 1024)
			}

			fmt.Printf("\r%s", renderProgressBar(currentSize, totalSize, speed, elapsed, eta, waiting))
		}
	}
}

func (c *GotdClient) SetWaiting(w bool) {
	c.mu.Lock()
	c.waiting = w
	c.mu.Unlock()
}

func (c *GotdClient) GetFileLocation(cacheKey string) (interface{}, bool) {
	c.fileLocMu.RLock()
	defer c.fileLocMu.RUnlock()
	loc, ok := c.fileLocations[cacheKey]
	return loc, ok
}

func (c *GotdClient) Disconnect() {}

func (c *GotdClient) GetChannelStatus(ctx context.Context, identifier string) (bool, int, time.Time, error) {
	active, channelID, accessHash, err := c.ResolveChannel(identifier)
	if err != nil {
		return false, 0, time.Time{}, err
	}
	if !active {
		return false, 0, time.Time{}, nil
	}

	msgs, err := c.GetMessages(ctx, channelID, accessHash, 1, 0)
	if err != nil || len(msgs) == 0 {
		return active, 0, time.Time{}, nil
	}

	return active, msgs[0].MessageID, msgs[0].Date, nil
}

func renderProgressBar(current, total int64, speed float64, elapsed, eta float64, waiting bool) string {
	width := 25
	percent := 0.0
	if total > 0 {
		percent = float64(current) / float64(total) * 100
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	etaStr := "--:--"
	if eta > 0 && eta < 86400 {
		h := int(eta) / 3600
		m := (int(eta) % 3600) / 60
		s := int(eta) % 60
		if h > 0 {
			etaStr = fmt.Sprintf("%dh%02dm", h, m)
		} else {
			etaStr = fmt.Sprintf("%dm%02ds", m, s)
		}
	}

	status := ""
	if waiting {
		status = " (Waiting)"
	}

	return fmt.Sprintf("  ↓ %s/%s [%s] %.0f%% %s %s %.1fMB/s%s",
		formatSize(current), formatSize(total), bar, percent,
		fmt.Sprintf("%ds", int(elapsed)), etaStr, speed, status)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func extractPasswordFromMessage(text string) string {
	lower := strings.ToLower(text)
	keywords := []string{"password found:", "password:", "pass:", "key:", "senha:"}
	for _, kw := range keywords {
		idx := strings.Index(lower, kw)
		if idx != -1 {
			after := text[idx+len(kw):]
			after = strings.TrimSpace(after)
			if len(after) > 0 {
				for i, ch := range after {
					if ch == ' ' || ch == '\n' || ch == '\r' || ch == '|' {
						return strings.TrimSpace(after[:i])
					}
				}
				return after
			}
		}
	}
	return ""
}

func isFloodWait(err error) bool {
	return FloodWaitDuration(err) > 0
}

// IsChannelError detecta erros que indicam canal deletado/privado/inacessível.
func IsChannelError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToUpper(err.Error())
	for _, msg := range []string{
		"CHANNEL_PRIVATE", "CHANNEL_INVALID", "PEER_ID_INVALID",
		"ACCESS_FORBIDDEN", "CHAT_NOT_FOUND", "USER_BANNED_IN_CHANNEL",
		"CHAT_ADMIN_REQUIRED", "CHANNEL_BANNED",
	} {
		if strings.Contains(s, msg) {
			return true
		}
	}
	return false
}

// FloodWaitDuration extrai os segundos de espera de um erro FLOOD_WAIT.
// Retorna 0 se não for FLOOD_WAIT.
func FloodWaitDuration(err error) time.Duration {
	if err == nil {
		return 0
	}
	s := err.Error()
	idx := strings.Index(s, "FLOOD_WAIT (")
	if idx == -1 {
		return 0
	}
	rest := s[idx+len("FLOOD_WAIT ("):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return 5 * time.Second
	}
	secs := 0
	for _, c := range rest[:end] {
		if c >= '0' && c <= '9' {
			secs = secs*10 + int(c-'0')
		}
	}
	if secs == 0 {
		return 5 * time.Second
	}
	return time.Duration(secs) * time.Second
}

type terminalAuth struct{ phone string }

func (t *terminalAuth) Phone(ctx context.Context) (string, error) {
	if t.phone != "" {
		return t.phone, nil
	}
	fmt.Print("Phone: ")
	var phone string
	fmt.Scanln(&phone)
	return phone, nil
}

func (t *terminalAuth) Code(ctx context.Context, sent *tg.AuthSentCode) (string, error) {
	fmt.Print("Code: ")
	var code string
	fmt.Scanln(&code)
	return code, nil
}

func (t *terminalAuth) Password(ctx context.Context) (string, error) {
	fmt.Print("2FA Password: ")
	var pass string
	fmt.Scanln(&pass)
	return pass, nil
}

func (t *terminalAuth) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (t *terminalAuth) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}
