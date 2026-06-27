package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/rs/zerolog"
)

func main() {
	loadDotEnv()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	apiID := os.Getenv("TG_API_ID")
	apiHash := os.Getenv("TG_API_HASH")
	phone := os.Getenv("TG_PHONE")
	sessionFile := os.Getenv("TG_SESSION_FILE")

	if apiID == "" || apiHash == "" {
		log.Fatal().Msg("Set TG_API_ID and TG_API_HASH in .env first")
	}
	if phone == "" {
		phone = prompt("Phone number (e.g. +5511999999999)")
	}
	if sessionFile == "" {
		sessionFile = "data/session.json"
	}

	var appID int
	fmt.Sscanf(apiID, "%d", &appID)

	log.Info().
		Str("phone", phone).
		Str("session", sessionFile).
		Msg("starting Telegram auth")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage := &session.FileStorage{Path: sessionFile}
	client := telegram.NewClient(appID, apiHash, telegram.Options{
		SessionStorage: storage,
	})

	err := client.Run(ctx, func(ctx context.Context) error {
		// Verificar se ja esta autenticado
		status, err := client.Auth().Status(ctx)
		if err == nil && status.Authorized {
			log.Info().Msg("already authenticated!")
			fmt.Println("\n=== AUTHENTICATED ===")
			fmt.Printf("Session saved to: %s\n", sessionFile)
			fmt.Println("Run: go run ./cmd/sulphurog/")
			cancel()
			return nil
		}

		log.Info().Msg("not authenticated, starting auth flow...")

		flow := auth.NewFlow(
			&terminalAuth{phone: phone},
			auth.SendCodeOptions{},
		)

		if err := flow.Run(ctx, client.Auth()); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}

		log.Info().Msg("authentication successful!")
		fmt.Println("\n=== AUTHENTICATED ===")
		fmt.Printf("Session saved to: %s\n", sessionFile)
		fmt.Println("Run: go run ./cmd/sulphurog/")

		cancel()
		return nil
	})

	if err != nil {
		log.Fatal().Err(err).Msg("client error")
	}
}

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

type terminalAuth struct {
	phone string
}

func (t *terminalAuth) Phone(ctx context.Context) (string, error) {
	if t.phone != "" {
		return t.phone, nil
	}
	return prompt("Phone number"), nil
}

func (t *terminalAuth) Code(ctx context.Context, sent *tg.AuthSentCode) (string, error) {
	fmt.Println("\nCode sent to your Telegram.")
	return prompt("Enter code"), nil
}

func (t *terminalAuth) Password(ctx context.Context) (string, error) {
	fmt.Println("\n2FA enabled on this account.")
	return prompt("Enter 2FA password"), nil
}

func (t *terminalAuth) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	fmt.Println("\nAccept Terms of Service? (y/n)")
	answer := prompt("Accept")
	if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		return fmt.Errorf("terms not accepted")
	}
	return nil
}

func (t *terminalAuth) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
