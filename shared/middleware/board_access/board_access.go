package board_access

import (
	"log"
	"sync"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

type Storage interface {
	GetBoardsAllowedEmails() ([]domain.Board, error)
}

type BoardAccess struct {
	data map[string][]string
	mu   sync.RWMutex
}

func New() *BoardAccess {
	return &BoardAccess{
		data: make(map[string][]string),
	}
}

func (b *BoardAccess) Update(s Storage) error {
	boards, err := s.GetBoardsAllowedEmails()
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Replace the entire map to avoid stale entries
	newData := make(map[string][]string)
	for _, board := range boards {
		newData[board.ShortName] = *board.AllowedEmails
	}
	b.data = newData

	return nil
}

func (b *BoardAccess) AllowedDomains(board string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.data[board]
}

func (b *BoardAccess) StartBackgroundUpdate(interval time.Duration, s Storage) {
	ticker := time.NewTicker(interval)
	log.Printf("Started BoardAccess background update")
	go func() {
		for range ticker.C {
			if err := b.Update(s); err != nil {
				log.Printf("Error updating board access rules: %v", err)
			}
		}
	}()
}
