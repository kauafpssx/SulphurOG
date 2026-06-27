package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Group struct {
	ID          string    `json:"id"`
	Identifier  string    `json:"identifier"`
	ChannelName string    `json:"channel_name"`
	ChannelID   int64     `json:"channel_id"`
	AccessHash  int64     `json:"access_hash"`
	Name        string    `json:"name"`
	Active      bool      `json:"active"`
	Dead        bool      `json:"dead"`
	Validated   bool      `json:"validated"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var (
	// https://t.me/canal ou https://t.me/+invite
	tgURLRegex = regexp.MustCompile(`(?:https?://)?t\.me/(?:\+)?([a-zA-Z0-9_]+)`)
	// -1001234567890 ou 1234567890
	tgIDRegex = regexp.MustCompile(`^-?100(\d{9,12}|\d{7,12})$|^(\d{5,12})$`)
)

// NormalizeIdentifier normaliza a entrada (URL ou ID)
func NormalizeIdentifier(input string) (identifier string, channelName string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("identifier is required")
	}

	// Limpar @ no inicio
	input = strings.TrimPrefix(input, "@")

	// Invite link: https://t.me/+hash
	if strings.Contains(input, "t.me/+") {
		parts := strings.Split(input, "t.me/+")
		if len(parts) > 1 {
			hash := strings.TrimSuffix(parts[1], "/")
			return fmt.Sprintf("https://t.me/+%s", hash), fmt.Sprintf("invite_%s", hash[:8]), nil
		}
	}

	// Tentar URL do Telegram
	if matches := tgURLRegex.FindStringSubmatch(input); matches != nil {
		name := matches[1]
		return fmt.Sprintf("https://t.me/%s", name), name, nil
	}

	// Tentar ID numerico
	if tgIDRegex.MatchString(input) {
		id, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return "", "", fmt.Errorf("invalid Telegram ID: %s", input)
		}
		return fmt.Sprintf("%d", id), "", nil
	}

	// Username (letras, numeros, underscore, 5-64 chars)
	if regexp.MustCompile(`^[a-zA-Z0-9_]{5,64}$`).MatchString(input) {
		return fmt.Sprintf("https://t.me/%s", input), input, nil
	}

	return "", "", fmt.Errorf("invalid identifier: %s (use URL, @username, invite link, or numeric ID)", input)
}

// IsID verifica se o identifier e um ID numerico
func (g *Group) IsID() bool {
	_, err := strconv.ParseInt(g.Identifier, 10, 64)
	return err == nil
}

// GetID retorna o ID numerico (se for URL, retorna 0)
func (g *Group) GetID() int64 {
	id, _ := strconv.ParseInt(g.Identifier, 10, 64)
	return id
}
