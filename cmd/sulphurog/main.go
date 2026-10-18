package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"strings"
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
	"github.com/sulphurog/sulphurog/internal/infrastructure/localstorage"
	"github.com/sulphurog/sulphurog/internal/infrastructure/tracker"
	tgclient "github.com/sulphurog/sulphurog/internal/infrastructure/telegram"
	"github.com/sulphurog/sulphurog/internal/usecase"
)

func main() {
	// Verificar flags de linha de comando para gerenciamento manual de grupos
	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--addgroup":
				if i+2 < len(os.Args) {
					identifier := os.Args[i+1]
					name := os.Args[i+2]
					addGroupCLI(identifier, name)
					os.Exit(0)
				}
			case "--updategroup":
				if i+3 < len(os.Args) {
					id := os.Args[i+1]
					identifier := os.Args[i+2]
					name := os.Args[i+3]
					updateGroupCLI(id, identifier, name)
					os.Exit(0)
				}
			case "--refresh":
				if err := refreshGroupsCLI(); err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}

	configPath := "configs/config.yaml"
	// Procurar primeiro argumento que não seja uma flag de grupo
	for i := 1; i < len(os.Args); i++ {
		if !strings.HasPrefix(os.Args[i], "--") {
			configPath = os.Args[i]
			break
		}
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
				// Verifica se há solicitação de reconexão por falha de RPC
				if tg.NeedsReconnect() {
					log.Info().Msg("reconnect requested due to RPC error")
					tg.ClearReconnectFlag()
					backoff = 5 * time.Second
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

		// Revalidar todos os grupos no startup (resolve canais recem-adicionados)
		if err := refreshGroupsCLI(); err != nil {
			log.Warn().Err(err).Msg("failed to refresh groups on startup")
		}
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

	// --- Local artifact storage ---
	artifactStorage, err := localstorage.New(cfg.Storage.OutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize local storage")
	}

	// --- Use Cases ---
	processFileUC := usecase.NewProcessFileUseCase(tg, zipExt, stealerParser, artifactStorage, trackerSvc, hashSvc, cfg.Processing.TempDir, cfg.Processing.ProcessCookies, log)
	_ = sevenZExt
	_ = detector
	monitorUC := usecase.NewMonitorGroupsUseCase(tg, processFileUC, groupRepo, trackerSvc, cfg.Processing.AllowedExtensions, log)

	// Iniciar monitor em background
	go monitorUC.Run(context.Background())

	// --- API ---
	// Gerar ou carregar API key (primeira inicialização gera uma nova)
	apiKey := cfg.API.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
	}
	if apiKey == "" {
		generated, err := api.LoadOrCreateAPIKey("data/api.key")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load or create API key")
		}
		apiKey = generated
		log.Info().Str("api_key", apiKey).Msg("API key gerada na primeira inicialização - salva em data/api.key")
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

	// Super rota mestre para manipulação de arquivos locais
	masterHandler := api.NewMasterRouteHandler(artifactStorage)
	masterHandler.RegisterRoutes(app)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", cfg.API.Port)
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

		// Filtrar erros de scanners/bots que enviam payloads binários TLS para a porta.
		// Não são bugs do app — apenas ruído de internet.
		errMsg := err.Error()
		if isScannerError(errMsg) {
			// Loga como debug em vez de error
			log.Debug().Str("remote", c.IP()).Msg("scanner request filtered")
			return c.Status(code).JSON(fiber.Map{"error": "bad request"})
		}

		log.Error().Err(err).Int("status", code).Msg("request error")
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
}

// isScannerError detecta erros causados por scanners/bots que enviam payloads
// binários não-HTTP para a porta (TLS, RocketMQ, etc).
func isScannerError(errMsg string) bool {
	scannerPatterns := []string{
		"unsupported http request method",
		"cannot find http request method",
		"error when reading request headers",
		"Buffer size=",
	}
	for _, pattern := range scannerPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// addGroupCLI adiciona um grupo via linha de comando (--addgroup identifier name).
func addGroupCLI(identifier, name string) {
	repo, err := repository.NewJSONGroupRepo("data/groups.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao inicializar repositorio: %v\n", err)
		os.Exit(1)
	}

	uc := usecase.NewManageGroupsUseCase(repo, nil, nil)
	group, err := uc.CreateGroup(identifier, name, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao adicionar grupo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Grupo adicionado com sucesso:\n")
	fmt.Printf("  ID:         %s\n", group.ID)
	fmt.Printf("  Identifier: %s\n", group.Identifier)
	fmt.Printf("  Name:       %s\n", group.Name)
}

// refreshGroupsCLI redefine todos os grupos como não validados, forçando o
// monitor a revalidá-los no próximo ciclo (útil quando o Telegram acaba de
// conectar ou o usuário acabou de aceitar convites de canais privados).
// Retorna erro em vez de sair do programa, para permitir uso no startup.
func refreshGroupsCLI() error {
	repo, err := repository.NewJSONGroupRepo("data/groups.json")
	if err != nil {
		return fmt.Errorf("erro ao inicializar repositorio: %w", err)
	}

	groups, err := repo.GetAll()
	if err != nil {
		return fmt.Errorf("erro ao buscar grupos: %w", err)
	}

	if len(groups) == 0 {
		fmt.Println("Nenhum grupo encontrado para refresh.")
		return nil
	}

	refreshed := 0
	for _, g := range groups {
		if !g.Validated || g.ChannelID == 0 {
			if err := repo.MarkUnvalidated(g.ID); err != nil {
				fmt.Fprintf(os.Stderr, "erro ao atualizar grupo %s: %v\n", g.ID, err)
				continue
			}
			refreshed++
		}
	}

	fmt.Printf("Refresh concluído: %d grupo(s) marcado(s) para revalidação.\n", refreshed)
	fmt.Println("O monitor revalidará os grupos no próximo ciclo (até ~60s).")
	return nil
}

// updateGroupCLI atualiza o identificador (URL) de um grupo existente.
// Útil quando o link do canal muda (ex: novo invite link).
func updateGroupCLI(id, identifier, name string) {
	repo, err := repository.NewJSONGroupRepo("data/groups.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro ao inicializar repositorio: %v\n", err)
		os.Exit(1)
	}

	if err := repo.UpdateIdentifier(id, identifier); err != nil {
		fmt.Fprintf(os.Stderr, "erro ao atualizar grupo: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Grupo atualizado com sucesso:\n")
	fmt.Printf("  ID:         %s\n", id)
	fmt.Printf("  Identifier: %s\n", identifier)
	fmt.Printf("  Name:       %s\n", name)
	fmt.Println("O monitor revalidará o grupo no próximo ciclo (até ~60s).")
}
