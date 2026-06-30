package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"

	"github.com/sulphurog/sulphurog/internal/infrastructure/api"
	"github.com/sulphurog/sulphurog/internal/infrastructure/extractor"
	"github.com/sulphurog/sulphurog/internal/infrastructure/hash"
	"github.com/sulphurog/sulphurog/internal/infrastructure/parser"
	"github.com/sulphurog/sulphurog/internal/infrastructure/repository"
	"github.com/sulphurog/sulphurog/internal/infrastructure/supabase"
	"github.com/sulphurog/sulphurog/internal/infrastructure/tracker"
	tgclient "github.com/sulphurog/sulphurog/internal/infrastructure/telegram"
	"github.com/sulphurog/sulphurog/internal/usecase"
)

func main() {
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Str("app", "sulphurog").
		Logger()

	log.Info().
		Str("config", configPath).
		Int("port", cfg.API.Port).
		Msg("starting sulphurog")

	// --- Infrastructure ---
	// Garantir que o diretório de dados existe antes de qualquer coisa
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create data dir")
	}

	groupRepo, err := repository.NewJSONGroupRepo("data/groups.json")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init group repo")
	}

	trackerSvc, err := tracker.NewSQLiteTracker("data/sulphurog.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init tracker")
	}

	hashSvc := hash.NewSHA256Service()

	// Telegram client (conectar em background)
	var tg *tgclient.GotdClient
	if cfg.TelegramAPIIDInt() > 0 && cfg.Telegram.APIHash != "" {
		tg = tgclient.NewGotdClient(
			cfg.TelegramAPIIDInt(),
			cfg.Telegram.APIHash,
			cfg.Telegram.Phone,
			cfg.Telegram.SessionFile,
			cfg.Processing.Threads,
			cfg.Processing.PartSizeKB,
			log,
		)

		go func() {
			backoff := 30 * time.Second
			for {
				log.Info().Msg("connecting to Telegram...")
				start := time.Now()
				err := tg.Connect(context.Background())
				if time.Since(start) > time.Minute {
					backoff = 30 * time.Second
				}
				if err != nil {
					log.Error().Err(err).Dur("retry_in", backoff).Msg("telegram disconnected")
				}
				log.Warn().Dur("retry_in", backoff).Msg("telegram reconnecting...")
				time.Sleep(backoff)
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			}
		}()

		// Esperar um pouco pra conexao estabelecer
		time.Sleep(3 * time.Second)
		log.Info().Msg("telegram client started")
	} else {
		log.Warn().Msg("telegram not configured, health checks will be limited")
	}

	// --- Extractors ---
	zipExt := extractor.NewZIPExtractor()
	sevenZExt := extractor.NewSevenZExtractor()
	detector := extractor.NewDetector()
	stealerParser := parser.NewStealerParser()

	// --- Auto-cleanup temp dir ---
	if cfg.Processing.TempDir != "" {
		if err := os.RemoveAll(cfg.Processing.TempDir); err != nil {
			log.Warn().Err(err).Msg("failed to cleanup temp dir")
		} else {
			os.MkdirAll(cfg.Processing.TempDir, 0755)
			log.Info().Str("dir", cfg.Processing.TempDir).Msg("temp dir cleaned")
		}
	}

	// --- Supabase ---
	supaClient := supabase.NewClient(cfg.Supabase.URL, cfg.Supabase.ServiceRoleKey)

	// --- Use Cases ---
	manageGroupsUC := usecase.NewManageGroupsUseCase(groupRepo, tg, trackerSvc)
	processFileUC := usecase.NewProcessFileUseCase(tg, zipExt, stealerParser, supaClient, trackerSvc, hashSvc, cfg.Processing.TempDir, cfg.Supabase.Bucket, cfg.Processing.ProcessCookies, log)
	_ = sevenZExt
	_ = detector
	monitorUC := usecase.NewMonitorGroupsUseCase(tg, processFileUC, groupRepo, trackerSvc, cfg.Processing.AllowedExtensions, log)

	// Iniciar monitor em background
	go monitorUC.Run(context.Background())

	// --- API ---
	apiKey := cfg.API.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
	}
	if apiKey == "" {
		apiKey = "dev-key-change-me"
	}

	app := fiber.New(fiber.Config{
		AppName:      "SulphurOG",
		ErrorHandler: customErrorHandler(log),
	})

	app.Use(recover.New())
	app.Use(logger.New())

	// Health check público (antes do auth)
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Use(api.APIKeyAuth(apiKey))

	// Groups CRUD
	groupsHandler := api.NewGroupsHandler(manageGroupsUC)
	groupsHandler.RegisterRoutes(app)

	// Status
	statusHandler := api.NewStatusHandler(manageGroupsUC, trackerSvc)
	statusHandler.RegisterRoutes(app)

	// Monitor control
	monitorCtrl := api.NewMonitorController()
	monitorCtrl.RegisterRoutes(app)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		addr := fmt.Sprintf(":%d", cfg.API.Port)
		log.Info().Str("addr", addr).Msg("listening")
		if err := app.Listen(addr); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")
	if tg != nil {
		tg.Disconnect()
	}
	if err := app.Shutdown(); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}

func customErrorHandler(log zerolog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}
		log.Error().Err(err).Int("status", code).Msg("request error")
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
}
