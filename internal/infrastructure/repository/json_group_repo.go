package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type JSONGroupRepository struct {
	filePath string
	mu       sync.RWMutex
	groups   map[string]*domain.Group
}

func NewJSONGroupRepo(filePath string) (*JSONGroupRepository, error) {
	repo := &JSONGroupRepository{
		filePath: filePath,
		groups:   make(map[string]*domain.Group),
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create group repo dir: %w", err)
	}

	if err := repo.load(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *JSONGroupRepository) load() error {
	data, err := os.ReadFile(r.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read groups file: %w", err)
	}

	var groups []*domain.Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return fmt.Errorf("failed to parse groups file: %w", err)
	}

	for _, g := range groups {
		r.groups[g.ID] = g
	}

	return nil
}

func (r *JSONGroupRepository) save() error {
	groups := make([]*domain.Group, 0, len(r.groups))
	for _, g := range r.groups {
		groups = append(groups, g)
	}

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal groups: %w", err)
	}

	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write groups tmp: %w", err)
	}

	if err := os.Rename(tmpPath, r.filePath); err != nil {
		return fmt.Errorf("failed to rename groups file: %w", err)
	}

	return nil
}

func (r *JSONGroupRepository) GetAll() ([]domain.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]domain.Group, 0, len(r.groups))
	for _, g := range r.groups {
		groups = append(groups, *g)
	}
	return groups, nil
}

func (r *JSONGroupRepository) GetByID(id string) (*domain.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return g, nil
}

func (r *JSONGroupRepository) Create(group *domain.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, g := range r.groups {
		if g.Identifier == group.Identifier {
			return fmt.Errorf("group with identifier already exists: %s", group.Identifier)
		}
	}

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	r.groups[group.ID] = group

	return r.save()
}

func (r *JSONGroupRepository) Update(group *domain.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.groups[group.ID]
	if !ok {
		return fmt.Errorf("group not found: %s", group.ID)
	}

	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = time.Now()

	r.groups[group.ID] = group

	return r.save()
}

func (r *JSONGroupRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.groups[id]; !ok {
		return fmt.Errorf("group not found: %s", id)
	}

	delete(r.groups, id)

	return r.save()
}
