package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sulphurog/sulphurog/internal/domain"
)

type ManageGroupsUseCase struct {
	groups   domain.GroupRepository
	telegram domain.TelegramClient
	tracker  domain.Tracker
}

func NewManageGroupsUseCase(
	groups domain.GroupRepository,
	telegram domain.TelegramClient,
	tracker domain.Tracker,
) *ManageGroupsUseCase {
	return &ManageGroupsUseCase{
		groups:   groups,
		telegram: telegram,
		tracker:  tracker,
	}
}

func (uc *ManageGroupsUseCase) ListGroups() ([]domain.Group, error) {
	return uc.groups.GetAll()
}

func (uc *ManageGroupsUseCase) GetGroup(id string) (*domain.Group, error) {
	return uc.groups.GetByID(id)
}

func (uc *ManageGroupsUseCase) CreateGroup(identifier string, name string, ignoreWithoutPassword bool) (*domain.Group, error) {
	normalized, channelName, err := domain.NormalizeIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	// Verificar duplicata
	existing, _ := uc.groups.GetAll()
	for _, g := range existing {
		if g.Identifier == normalized {
			return nil, fmt.Errorf("group already exists: %s", normalized)
		}
	}

	group := &domain.Group{
		ID:                   uuid.New().String()[:8],
		Identifier:           normalized,
		ChannelName:          channelName,
		Name:                 name,
		Active:               true,
		Validated:            false,
		IgnoreWithoutPassword: ignoreWithoutPassword,
	}

	// Se telegram client existe e esta pronto, validar o canal
	if uc.telegram != nil {
		active, channelID, accessHash, err := uc.telegram.ResolveChannel(normalized)
		if err != nil {
			// Nao bloquear se Telegram nao conectou ainda
			group.Validated = false
			group.Active = true
		} else {
			group.ChannelID = channelID
			group.AccessHash = accessHash
			group.Validated = true
			group.Active = active
		}
	}

	if err := uc.groups.Create(group); err != nil {
		return nil, err
	}

	return group, nil
}

func (uc *ManageGroupsUseCase) UpdateGroup(id string, identifier *string, name *string, active *bool, dead *bool, ignoreWithoutPassword *bool) (*domain.Group, error) {
	group, err := uc.groups.GetByID(id)
	if err != nil {
		return nil, err
	}

	if identifier != nil {
		normalized, channelName, err := domain.NormalizeIdentifier(*identifier)
		if err != nil {
			return nil, err
		}
		group.Identifier = normalized
		group.ChannelName = channelName
		group.Validated = false

		// Re-validar se telegram client existe e pronto
		if uc.telegram != nil {
			active, channelID, accessHash, err := uc.telegram.ResolveChannel(normalized)
			if err == nil {
				group.ChannelID = channelID
				group.AccessHash = accessHash
				group.Validated = true
				group.Active = active
			}
		}
	}
	if name != nil {
		group.Name = *name
	}
	if active != nil {
		group.Active = *active
	}
	if dead != nil {
		group.Dead = *dead
		if *dead {
			group.Active = false
		}
	}
	if ignoreWithoutPassword != nil {
		group.IgnoreWithoutPassword = *ignoreWithoutPassword
	}

	if err := uc.groups.Update(group); err != nil {
		return nil, err
	}

	return group, nil
}

func (uc *ManageGroupsUseCase) DeleteGroup(id string) error {
	return uc.groups.Delete(id)
}

type HealthResult struct {
	Active          bool      `json:"active"`
	Dead            bool      `json:"dead"`
	Validated       bool      `json:"validated"`
	ChannelID       int64     `json:"channel_id,omitempty"`
	ChannelName     string    `json:"channel_name,omitempty"`
	LastMessageID   int       `json:"last_message_id"`
	LastMessageDate time.Time `json:"last_message_date,omitempty"`
	TotalDownloaded int       `json:"total_downloaded"`
	TotalFailed     int       `json:"total_failed"`
	LastCheck       time.Time `json:"last_check"`
	Error           string    `json:"error,omitempty"`
}

func (uc *ManageGroupsUseCase) CheckHealth(groupID string) (*HealthResult, error) {
	group, err := uc.groups.GetByID(groupID)
	if err != nil {
		return nil, err
	}

	result := &HealthResult{
		Active:      group.Active,
		Dead:        group.Dead,
		Validated:   group.Validated,
		ChannelID:   group.ChannelID,
		ChannelName: group.ChannelName,
	}

	if uc.tracker != nil {
		state, err := uc.tracker.GetGroupState(group.Identifier)
		if err == nil && state != nil {
			result.LastMessageID = state.LastMessageID
			result.TotalDownloaded = state.TotalDownloaded
			result.TotalFailed = state.TotalFailed
			result.LastCheck = state.LastCheck
		}
	}

	// Validar com Telegram real
	if uc.telegram != nil {
		ctx := context.Background()
		active, lastID, lastDate, err := uc.telegram.GetChannelStatus(ctx, group.Identifier)
		if err != nil {
			result.Error = err.Error()
			result.Active = false
		} else {
			result.Active = active
			result.LastMessageID = lastID
			result.LastMessageDate = lastDate
			result.Validated = true

			// Atualizar grupo no repo
			group.Active = active
			group.Validated = true
			group.ChannelID = group.ChannelID // manter se ja tinha
			uc.groups.Update(group)
		}
	}

	return result, nil
}
