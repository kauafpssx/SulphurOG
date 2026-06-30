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
	groupValidationInterval = 10
	maxConsecFails          = 3
)

type MonitorGroupsUseCase struct {
	telegram    domain.TelegramClient
	processor   *ProcessFileUseCase
	groups      domain.GroupRepository
	tracker     domain.Tracker
	log         zerolog.Logger
	cycleCount  int
	groupsCache []domain.Group
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

type FileLocationProvider interface {
	GetFileLocation(cacheKey string) (interface{}, bool)
}

func (uc *MonitorGroupsUseCase) Run(ctx context.Context) {
	uc.log.Info().Msg("monitor started")
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
		uc.groupsCache = allGroups

		// Validação periódica de grupos vivos
		if uc.cycleCount%groupValidationInterval == 1 {
			uc.validateGroups(ctx, allGroups)
			if fresh, err := uc.groups.GetAll(); err == nil {
				allGroups = fresh
			}
		}

		// Fase 1: escaneia TODOS os grupos e enfileira arquivos novos
		for _, group := range allGroups {
			if !group.Active || group.Dead || !group.Validated || group.ChannelID == 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			uc.enqueueGroup(ctx, group)
			time.Sleep(5 * time.Second)
		}

		// Fase 2: processa até 10 arquivos por ciclo, depois volta pro Phase 1
		processed := 0
		for processed < 10 {
			select {
			case <-ctx.Done():
				return
			default:
			}

			empty, retErr := uc.processNextFile(ctx)
			if retErr != nil {
				if errors.Is(retErr, domain.ErrStorageFull) {
					uc.log.Warn().Msg("bucket full — pausing 30min")
					time.Sleep(30 * time.Minute)
					break
				}
			}
			if empty {
				break
			}
			processed++
			time.Sleep(5 * time.Second)
		}

		uc.log.Info().Int("cycle", uc.cycleCount).Int("processed", processed).Msg("cycle complete, waiting 60s...")
		time.Sleep(60 * time.Second)
	}
}

// enqueueGroup escaneia um grupo e adiciona arquivos novos na fila global.
// Não processa nada — só enfileira.
func (uc *MonitorGroupsUseCase) enqueueGroup(ctx context.Context, group domain.Group) {
	if uc.telegram == nil {
		return
	}

	log := uc.log.With().Str("group", group.Name).Str("id", group.ID).Logger()

	groupState, err := uc.tracker.GetGroupState(group.Identifier)
	if err != nil || groupState == nil {
		groupState = &domain.GroupState{}
	}

	// Salva estado anterior para decidir se precisa buscar histórico
	previousOldestID := groupState.OldestMessageID

	var pending []domain.PendingFile
	skipped := 0

	// Mensagens recentes
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
		if f.Password == "" && group.IgnoreWithoutPassword {
			skipped++
			continue
		}
		if f.MessageID <= groupState.LastMessageID {
			continue
		}
		if dup := uc.isDuplicate(f.SourceURL, f.Filename, f.FileSize); dup {
			log.Debug().Str("file", f.Filename).Msg("duplicate skipped")
			continue
		}
		log.Info().Str("file", f.Filename).Msg("new file found")
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

	for _, f := range recentFiles {
		if f.MessageID > groupState.LastMessageID {
			groupState.LastMessageID = f.MessageID
		}
		if groupState.OldestMessageID == 0 || f.MessageID < groupState.OldestMessageID {
			groupState.OldestMessageID = f.MessageID
		}
	}

	// Histórico: busca mais atrás apenas na primeira vez que vemos o grupo
	// Grupos já rastreados só precisam dos recentes
	time.Sleep(2 * time.Second)
	if previousOldestID == 0 && groupState.OldestMessageID > 0 {
		var histFiles []domain.LogFile
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			histFiles, err = uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 10, groupState.OldestMessageID)
			if err == nil {
				break
			}
			if wait := tgclient.FloodWaitDuration(err); wait > 0 {
				log.Warn().Dur("wait", wait).Int("attempt", attempt+1).Msg("FLOOD_WAIT on historical fetch, waiting...")
				time.Sleep(wait)
				continue
			}
			break
		}
		if err != nil {
			log.Warn().Err(err).Msg("failed to list historical files, skipping")
		} else {
			log.Info().Int("count", len(histFiles)).Msg("historical files found")
		for _, f := range histFiles {
			if f.Password == "" && group.IgnoreWithoutPassword {
				skipped++
				continue
			}
				if dup := uc.isDuplicate(f.SourceURL, f.Filename, f.FileSize); dup {
					log.Debug().Str("file", f.Filename).Msg("duplicate skipped")
					continue
				}
				log.Info().Str("file", f.Filename).Msg("historical file found")
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
				if f.MessageID < groupState.OldestMessageID {
					groupState.OldestMessageID = f.MessageID
				}
			}
		}
	}

	if skipped > 0 {
		log.Debug().Int("skipped", skipped).Msg("files skipped (no password or duplicate)")
	}

	if len(pending) > 0 {
		if err := uc.tracker.AddPending(pending); err != nil {
			log.Error().Err(err).Msg("failed to add pending")
		} else {
			log.Info().Int("count", len(pending)).Msg("files added to queue")
		}
	}

	uc.tracker.UpdateGroupState(group.Identifier, *groupState)
}

// processNextFile pega o arquivo mais recente da fila global e processa.
// Retorna (true, nil) quando a fila está vazia.
func (uc *MonitorGroupsUseCase) processNextFile(ctx context.Context) (queueEmpty bool, retErr error) {
	pendingFiles, err := uc.tracker.GetPending(1)
	if err != nil || len(pendingFiles) == 0 {
		return true, nil
	}

	file := pendingFiles[0]

	// Resolve grupo pelo identifier
	group := uc.groupByIdentifier(file.Group)
	if group == nil || group.Dead || !group.Active {
		uc.log.Warn().Str("group", file.Group).Msg("group dead/missing, purging its queue")
		uc.tracker.RemovePendingByGroup(file.Group)
		return false, nil
	}

	// Já processado com sucesso?
	if already, _ := uc.tracker.IsDownloaded(file.Source); already {
		uc.tracker.RemovePending(file.Source)
		return false, nil
	}

	// Resolver file location do cache
	var fileLocation interface{}
	if provider, ok := uc.telegram.(FileLocationProvider); ok {
		cacheKey := fmt.Sprintf("%d_%d", group.ChannelID, file.MessageID)
		if loc, found := provider.GetFileLocation(cacheKey); found {
			fileLocation = loc
		} else {
			// Cache miss (ex: após reconnect) — re-fetch
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
				uc.log.Warn().Str("file", file.Filename).Msg("could not get file location, skipping")
				uc.tracker.RemovePending(file.Source)
				return false, nil
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

	uc.log.Info().
		Str("group", group.Name).
		Str("file", file.Filename).
		Int64("size_mb", file.FileSize/1024/1024).
		Msg("downloading")

	if err := uc.processor.Execute(ctx, logFile); err != nil {
		if errors.Is(err, domain.ErrStorageFull) {
			return false, domain.ErrStorageFull
		}
		if tgclient.IsChannelError(err) {
			uc.log.Warn().Err(err).Str("group", group.Name).Msg("channel error, marking dead")
			state, _ := uc.tracker.GetGroupState(group.Identifier)
			if state == nil {
				state = &domain.GroupState{}
			}
			state.ConsecFails = maxConsecFails
			uc.killGroup(ctx, *group, *state, err.Error())
			return false, nil
		}
		uc.log.Error().Err(err).Str("file", file.Filename).Msg("process failed")
	}

	uc.tracker.RemovePending(file.Source)
	return false, nil
}

// isDuplicate verifica source URL, filename+size e tracker.
func (uc *MonitorGroupsUseCase) isDuplicate(sourceURL, filename string, fileSize int64) bool {
	if already, _ := uc.tracker.IsDownloaded(sourceURL); already {
		return true
	}
	if dup, _ := uc.tracker.IsDuplicateFile(filename, fileSize); dup {
		return true
	}
	return false
}

// groupByIdentifier busca grupo no cache (atualizado a cada ciclo).
func (uc *MonitorGroupsUseCase) groupByIdentifier(identifier string) *domain.Group {
	for i := range uc.groupsCache {
		if uc.groupsCache[i].Identifier == identifier {
			return &uc.groupsCache[i]
		}
	}
	return nil
}

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
			log.Warn().Str("group", group.Name).Int("consec_fails", state.ConsecFails).Err(err).Msg("group unreachable")
			if state.ConsecFails >= maxConsecFails {
				uc.killGroup(ctx, group, *state, fmt.Sprintf("unreachable after %d checks", state.ConsecFails))
			} else {
				uc.tracker.UpdateGroupState(group.Identifier, *state)
			}
		} else {
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
