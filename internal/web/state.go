package web

import (
	"sort"
	"sync"

	"alphacore/internal/models"
)

type StateManager struct {
	sync.RWMutex
	data map[string]models.IndexResult
}

func NewStateManager() *StateManager {
	return &StateManager{
		data: make(map[string]models.IndexResult),
	}
}

// UpdateBatch updates the state with a new batch of real-time results
func (sm *StateManager) UpdateBatch(batch []models.IndexResult) {
	sm.Lock()
	defer sm.Unlock()
	for _, result := range batch {
		sm.data[result.IndexCode] = result
	}
}

// GetAllSorted returns a snapshot of the latest data sorted by ETF code ascending
func (sm *StateManager) GetAllSorted() []models.IndexResult {
	sm.RLock()
	defer sm.RUnlock()

	var list []models.IndexResult
	for _, v := range sm.data {
		list = append(list, v)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].IndexCode < list[j].IndexCode
	})

	return list
}
