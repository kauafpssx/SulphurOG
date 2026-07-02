package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	allowedExt  map[string]bool
}

func NewMonitorGroupsUseCase(
	telegram domain.TelegramClient,
	processor *ProcessFileUseCase,
	groups domain.GroupRepository,
	tracker domain.Tracker,
	allowedExts []string,
	log zerolog.Logger,
) *MonitorGroupsUseCase {
	extMap := make(map[string]bool)
	for _, ext := range allowedExts {
		extMap[strings.ToLower(ext)] = true
	}
	if len(extMap) == 0 {
		extMap[".zip"] = true
		extMap[".rar"] = true
		extMap[".7z"] = true
		extMap[".gz"] = true
		extMap[".txt"] = true
	}
	return &MonitorGroupsUseCase{
		telegram:  telegram,
		processor: processor,
		groups:    groups,
		tracker:   tracker,
		log:       log,
		allowedExt: extMap,
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
	// Coleta FLOOD_WAITs e dorme o tempo necessário antes de retry
	var floodWaitGroups []domain.Group

	for _, group := range allGroups {
		if !group.Active || group.Dead || !group.Validated || group.ChannelID == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		flooded := uc.enqueueGroup(ctx, group)
		if flooded {
			floodWaitGroups = append(floodWaitGroups, group)
		}
		time.Sleep(5 * time.Second)
	}

	// Se teve FLOOD_WAIT, dorme o tempo máximo e retry os grupos pulados
	if len(floodWaitGroups) > 0 {
		// Pega o maior FLOOD_WAIT dos grupos pulados
		var maxFloodWait time.Time
		for _, g := range floodWaitGroups {
			state, err := uc.tracker.GetGroupState(g.Identifier)
			if err == nil && state != nil && state.FloodWaitUntil.After(maxFloodWait) {
				maxFloodWait = state.FloodWaitUntil
			}
		}
		if !maxFloodWait.IsZero() {
			wait := time.Until(maxFloodWait)
			if wait > 0 {
				uc.log.Info().Dur("wait", wait).Int("groups", len(floodWaitGroups)).Msg("FLOOD_WAIT: sleeping before retry")
				time.Sleep(wait)
			}
			uc.log.Info().Int("groups", len(floodWaitGroups)).Msg("retrying groups after FLOOD_WAIT")
			for _, group := range floodWaitGroups {
				select {
				case <-ctx.Done():
					return
				default:
				}
				uc.enqueueGroup(ctx, group)
				time.Sleep(5 * time.Second)
			}
		}
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
// Retorna true se o grupo foi pausado por FLOOD_WAIT.
func (uc *MonitorGroupsUseCase) enqueueGroup(ctx context.Context, group domain.Group) bool {
	if uc.telegram == nil {
		return false
	}

	log := uc.log.With().Str("group", group.Name).Str("id", group.ID).Logger()

	groupState, err := uc.tracker.GetGroupState(group.Identifier)
	if err != nil || groupState == nil {
		groupState = &domain.GroupState{}
	}

	var pending []domain.PendingFile
	skipped := 0
	isFirstRun := groupState.LastMessageID == 0

	// Busca 10 mensagens mais recentes do grupo
	recentFiles, err := uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 10, 0)
	if err != nil {
		if tgclient.IsChannelError(err) {
			log.Warn().Err(err).Msg("channel inaccessible, marking dead")
			groupState.ConsecFails = maxConsecFails
			uc.killGroup(ctx, group, *groupState, err.Error())
			return false
		}
		if waitDur := tgclient.FloodWaitDuration(err); waitDur > 0 {
			groupState.FloodWaitUntil = time.Now().Add(waitDur)
			uc.tracker.UpdateGroupState(group.Identifier, *groupState)
			log.Warn().Dur("wait", waitDur).Msg("FLOOD_WAIT, will retry after wait")
			return true
		}
		log.Error().Err(err).Msg("failed to list recent files")
		return false
	}
	// Scan OK — limpa FLOOD_WAIT anterior
	groupState.FloodWaitUntil = time.Time{}
	log.Info().Int("count", len(recentFiles)).Msg("recent files found")

	for _, f := range recentFiles {
		// Filtros
		if f.Password == "" && group.IgnoreWithoutPassword {
			skipped++
			continue
		}
		if !uc.isAllowedExtension(f.Filename) {
			skipped++
			continue
		}

		// Primeiro run: adiciona todos (marca posição, não perde nada)
		// Runs seguintes: só novos (mais recentes que último visto)
		if isFirstRun {
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
		} else {
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
	}

	// Atualiza LastMessageID com o ID mais alto retornado
	for _, f := range recentFiles {
		if f.MessageID > groupState.LastMessageID {
			groupState.LastMessageID = f.MessageID
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
	return false
}

// processNextFile pega o arquivo mais recente da fila global e processa.
// Retorna (true, nil) quando a fila está vazia.
func (uc *MonitorGroupsUseCase) resolveFileLocation(ctx context.Context, group *domain.Group, file domain.PendingFile) (interface{}, bool) {
	// Sempre re-fetch file location para evitar FILE_REFERENCE_EXPIRED (Fix F)
	msgs, fetchErr := uc.telegram.ListFiles(ctx, group.ChannelID, group.AccessHash, 5, file.MessageID+1)
	if fetchErr == nil {
		for _, m := range msgs {
			if m.MessageID == file.MessageID && m.FileID == file.FileID && m.FileLocation != nil {
				return m.FileLocation, true
			}
		}
	}
	// Fallback: tenta cache
	if provider, ok := uc.telegram.(FileLocationProvider); ok {
		cacheKey := fmt.Sprintf("%d_%d_%s", group.ChannelID, file.MessageID, file.FileID)
		if loc, found := provider.GetFileLocation(cacheKey); found {
			return loc, true
		}
	}
	return nil, false
}

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

	// Verifica se excedeu limite de retries
	if rec, err := uc.tracker.GetFileRecord(file.Source); err == nil && rec.Retries >= 3 {
		uc.log.Warn().Str("file", file.Filename).Int("retries", rec.Retries).Msg("max retries exceeded, removing from queue")
		uc.tracker.RemovePending(file.Source)
		return false, nil
	}

	fileLocation, ok := uc.resolveFileLocation(ctx, group, file)
	if !ok {
		uc.log.Warn().Str("file", file.Filename).Msg("could not get file location, skipping")
		uc.tracker.RemovePending(file.Source)
		return false, nil
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
		// Erro transiente (FILE_REFERENCE_EXPIRED, OFFSET_INVALID, etc.): tenta uma vez (Fix A2)
		if tgclient.IsTransientDownloadError(err) {
			uc.log.Warn().Str("file", file.Filename).Err(err).Msg("transient error, retrying with fresh reference")
			if newLoc, ok := uc.resolveFileLocation(ctx, group, file); ok {
				logFile.FileLocation = newLoc
				retryErr := uc.processor.Execute(ctx, logFile)
				if retryErr == nil {
					uc.tracker.RemovePending(file.Source)
					return false, nil
				}
				if tgclient.IsTransientDownloadError(retryErr) {
					uc.log.Warn().Str("file", file.Filename).Msg("retry also failed with transient error, skipping")
				} else {
					uc.log.Error().Err(retryErr).Str("file", file.Filename).Msg("retry failed with different error")
				}
			} else {
				uc.log.Warn().Str("file", file.Filename).Msg("could not refresh file location for retry, skipping")
			}
			uc.tracker.RemovePending(file.Source)
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

// isAllowedExtension verifica se o arquivo tem extensão permitida.
func (uc *MonitorGroupsUseCase) isAllowedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return uc.allowedExt[ext]
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
