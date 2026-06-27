package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/sulphurog/sulphurog/internal/domain"
	tgclient "github.com/sulphurog/sulphurog/internal/infrastructure/telegram"
)

const (
	// A cada quantos ciclos valida se os grupos ainda estão vivos
	groupValidationInterval = 10
	// Falhas consecutivas antes de marcar grupo como morto
	maxConsecFails = 3
)

type MonitorGroupsUseCase struct {
	telegram   domain.TelegramClient
	processor  *ProcessFileUseCase
	groups     domain.GroupRepository
	tracker    domain.Tracker
	log        zerolog.Logger
	cycleCount int
}

func NewMonitorGroupsUseCase(
	telegram domain.TelegramClient,
	processor *ProcessFileUseCase,
	groups domain.GroupRepository,
	tracker domain.Tracker,
	log zerolog.Logger,
) *MonitorGroupsUseCase {
	return &MonitorGroupsUseCase{
		telegram:  telegram,
		processor: processor,
		groups:    groups,
		tracker:   tracker,
		log:       log,
	}
}

// FileLocationProvider interface pra obter file location do cache
type FileLocationProvider interface {
	GetFileLocation(cacheKey string) (interface{}, bool)
}

func (uc *MonitorGroupsUseCase) Run(ctx context.Context) {
	uc.log.Info().Msg("monitor started")

	// Esperar conexão Telegram estabelecer
	time.Sleep(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			uc.log.Info().Msg("monitor stopped")
			return
		default:
		}

		uc.cycleCount++

		allGroups, err := uc.groups.GetAll()
		if err != nil {
			uc.log.Error().Err(err).Msg("failed to get groups")
			time.Sleep(30 * time.Second)
			continue
		}

		// Validação periódica de grupos (a cada N ciclos)
		if uc.cycleCount%groupValidationInterval == 1 {
			uc.validateGroups(ctx, allGroups)
			// Recarregar grupos pois validação pode ter marcado dead
			if fresh, err := uc.groups.GetAll(); err == nil {
				allGroups = fresh
			}
		}

		for _, group := range allGroups {
			if !group.Active || group.Dead || !group.Validated || group.ChannelID == 0 {
				continue
			}

			select {
			case <-ctx.Done():
				return
			default:
			}

			uc.processGroup(ctx, group)
			time.Sleep(10 * time.Second)
		}

		uc.log.Info().Int("cycle", uc.cycleCount).Msg("cycle complete, waiting 60s...")
		time.Sleep(60 * time.Second)
	}
}

// validateGroups checa se cada grupo ainda está acessível no Telegram.
// Após maxConsecFails consecutivas falhas, marca o grupo como morto e limpa a fila.
func (uc *MonitorGroupsUseCase) validateGroups(ctx context.Context, groups []domain.Group) {
	if uc.telegram == nil {
		return
	}
	log := uc.log.With().Str("op", "validate").Logger()
	log.Info().Int("groups", len(groups)).Msg("validating groups...")

	for _, group := range groups {
		if !group.Active || !group.Validated || group.ChannelID == 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		state, err := uc.tracker.GetGroupState(group.Identifier)
		if err != nil || state == nil {
			state = &domain.GroupState{}
		}

		active, _, _, err := uc.telegram.ResolveChannel(group.Identifier)
		if err != nil || !active {
			state.ConsecFails++
			log.Warn().
				Str("group", group.Name).
				Int("consec_fails", state.ConsecFails).
				Err(err).
				Msg("group unreachable")

			if state.ConsecFails >= maxConsecFails {
				uc.killGroup(ctx, group, *state, fmt.Sprintf("unreachable after %d checks", state.ConsecFails))
			} else {
				uc.tracker.UpdateGroupState(group.Identifier, *state)
			}
		} else {
			// Reset contador de falhas
			if state.ConsecFails > 0 {
				state.ConsecFails = 0
				log.Info().Str("group", group.Name).Msg("group recovered")
			}
			state.LastValidated = time.Now()
			uc.tracker.UpdateGroupState(group.Identifier, *state)
		}

		time.Sleep(3 * time.Second)
	}
}

// killGroup marca grupo como morto e limpa fila pendente.
func (uc *MonitorGroupsUseCase) killGroup(ctx context.Context, group domain.Group, state domain.GroupState, reason string) {
	log := uc.log.With().Str("group", group.Name).Logger()
	log.Warn().Str("reason", reason).Msg("marking group as dead, purging pending queue")

	group.Dead = true
	group.Active = false
	group.UpdatedAt = time.Now()
	if err := uc.groups.Update(&group); err != nil {
		log.Error().Err(err).Msg("failed to mark group dead")
	}

	if err := uc.tracker.RemovePendingByGroup(group.Identifier); err != nil {
		log.Error().Err(err).Msg("failed to purge pending for dead group")
	}

	uc.tracker.UpdateGroupState(group.Identifier, state)
}

func (uc *MonitorGroupsUseCase) processGroup(ctx context.Context, group domain.Group) {
	if uc.telegram == nil {
		return
	}

	log := uc.log.With().Str("group", group.Name).Str("id", group.ID).Logger()
	log.Info().Msg("processing group")

	groupState, err := uc.tracker.GetGroupState(group.Identifier)
	if err != nil || groupState == nil {
		groupState = &domain.GroupState{}
	}

	var pending []domain.PendingFile
	skipped := 0

	// 1. Buscar mensagens mais recentes
	recentFiles, err := uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 10, 0)
	if err != nil {
		if tgclient.IsChannelError(err) {
			log.Warn().Err(err).Msg("channel inaccessible, marking dead")
			groupState.ConsecFails = maxConsecFails
			uc.killGroup(ctx, group, *groupState, err.Error())
			return
		}
		log.Error().Err(err).Msg("failed to list recent files")
		return
	}
	log.Info().Int("count", len(recentFiles)).Msg("recent files found")

	for _, f := range recentFiles {
		if f.Password == "" {
			skipped++
			continue
		}
		// Só enfileira se é genuinamente novo (ID maior que o ultimo visto)
		if f.MessageID > groupState.LastMessageID {
			log.Info().Str("file", f.Filename).Str("password", f.Password).Msg("new file found")
			pending = append(pending, domain.PendingFile{
				MessageID: f.MessageID,
				FileID:    f.FileID,
				Source:    f.SourceURL,
				Group:     group.Identifier,
				Filename:  f.Filename,
				FileSize:  f.FileSize,
				Date:      f.Date,
				Priority:  1,
				Password:  f.Password,
			})
		}
	}

	// Atualizar LastMessageID e OldestMessageID a partir dos arquivos recentes
	for _, f := range recentFiles {
		if f.MessageID > groupState.LastMessageID {
			groupState.LastMessageID = f.MessageID
		}
		if groupState.OldestMessageID == 0 || f.MessageID < groupState.OldestMessageID {
			groupState.OldestMessageID = f.MessageID
		}
	}

	// 2. Paginar historico (arquivos mais antigos que o menor ID ja visto)
	// Pequeno delay pra nao bater FLOOD_WAIT logo apos o fetch recente
	time.Sleep(2 * time.Second)

	if groupState.OldestMessageID > 0 {
		historicalFiles, err := uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 10, groupState.OldestMessageID)
		if err != nil {
			if wait := tgclient.FloodWaitDuration(err); wait > 0 {
				log.Warn().Dur("wait", wait).Msg("FLOOD_WAIT on historical fetch, waiting...")
				time.Sleep(wait)
				historicalFiles, err = uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 10, groupState.OldestMessageID)
			}
		}
		if err != nil {
			log.Warn().Err(err).Msg("failed to list historical files, skipping")
		} else {
			log.Info().Int("count", len(historicalFiles)).Msg("historical files found")
			for _, f := range historicalFiles {
				if f.Password == "" {
					skipped++
					continue
				}
				// Pula se ja foi baixado
				if already, _ := uc.tracker.IsDownloaded(f.SourceURL); already {
					continue
				}
				log.Info().Str("file", f.Filename).Str("password", f.Password).Msg("historical file found")
				pending = append(pending, domain.PendingFile{
					MessageID: f.MessageID,
					FileID:    f.FileID,
					Source:    f.SourceURL,
					Group:     group.Identifier,
					Filename:  f.Filename,
					FileSize:  f.FileSize,
					Date:      f.Date,
					Priority:  2,
					Password:  f.Password,
				})
				// Avança o ponteiro de paginacao historica
				if f.MessageID < groupState.OldestMessageID {
					groupState.OldestMessageID = f.MessageID
				}
			}
		}
	}

	if skipped > 0 {
		log.Info().Int("skipped", skipped).Msg("files skipped (no password)")
	}

	if len(pending) > 0 {
		if err := uc.tracker.AddPending(pending); err != nil {
			log.Error().Err(err).Msg("failed to add pending")
			return
		}
		log.Info().Int("count", len(pending)).Msg("files added to queue")
	}

	// Processar fila (1 por vez)
	processed := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pendingFiles, err := uc.tracker.GetPending(1)
		if err != nil || len(pendingFiles) == 0 {
			break
		}

		file := pendingFiles[0]

		// Grupo foi deletado enquanto processava — limpa toda a fila do grupo
		if g, err := uc.groups.GetByID(file.Group); err == nil && g != nil && (g.Dead || !g.Active) {
			log.Warn().Str("group", file.Group).Msg("group dead/inactive, purging its pending files")
			uc.tracker.RemovePendingByGroup(file.Group)
			break
		}

		// Pula se ja foi baixado com sucesso
		if already, _ := uc.tracker.IsDownloaded(file.Source); already {
			uc.tracker.RemovePending(file.Source)
			continue
		}

		// Buscar file location do cache — se não tiver, tentar re-fetch via GetMessages
		var fileLocation interface{}
		if provider, ok := uc.telegram.(FileLocationProvider); ok {
			cacheKey := fmt.Sprintf("%d_%d", group.ChannelID, file.MessageID)
			if loc, found := provider.GetFileLocation(cacheKey); found {
				fileLocation = loc
				log.Debug().Str("key", cacheKey).Msg("file location from cache")
			} else {
				// Cache miss (ex: após reconnect) — re-fetch
				log.Debug().Str("key", cacheKey).Msg("cache miss, re-fetching location")
				msgs, fetchErr := uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 5, file.MessageID+1)
				if fetchErr == nil {
					for _, m := range msgs {
						if m.MessageID == file.MessageID && m.FileLocation != nil {
							fileLocation = m.FileLocation
							break
						}
					}
				}
				if fileLocation == nil {
					log.Warn().Str("file", file.Filename).Msg("could not get file location, skipping")
					uc.tracker.RemovePending(file.Source)
					continue
				}
			}
		}

		logFile := domain.LogFile{
			ID:           file.Source,
			MessageID:    file.MessageID,
			FileID:       file.FileID,
			SourceURL:    file.Source,
			Filename:     file.Filename,
			FileSize:     file.FileSize,
			Date:         file.Date,
			ContentHash:  file.Source,
			FileLocation: fileLocation,
			Password:     file.Password,
		}

		log.Info().Str("file", file.Filename).Int64("size_mb", file.FileSize/1024/1024).Msg("downloading")

		if err := uc.processor.Execute(ctx, logFile); err != nil {
			if errors.Is(err, domain.ErrStorageFull) {
				log.Warn().Msg("bucket full — pausing 30min, file stays in queue")
				time.Sleep(30 * time.Minute)
				break
			}
			if tgclient.IsChannelError(err) {
				log.Warn().Err(err).Str("group", group.Name).Msg("channel error during download, marking group dead")
				groupState.ConsecFails = maxConsecFails
				uc.killGroup(ctx, group, *groupState, err.Error())
				break
			}
			log.Error().Err(err).Str("file", file.Filename).Msg("process failed")
		}

		uc.tracker.RemovePending(file.Source)
		processed++
		time.Sleep(5 * time.Second)
	}

	uc.tracker.UpdateGroupState(group.Identifier, *groupState)
	log.Info().Int("processed", processed).Msg("group processing complete")
}
