package pg

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

func (s *Storage) StartPeriodicViewRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				boards, err := s.GetActiveBoards(interval)
				if err != nil {
					log.Printf("Error fetching active boards: %v", err)
					continue
				}
				var wg sync.WaitGroup
				for _, board := range boards {
					wg.Add(1)
					go func(b domain.Board) {
						defer wg.Done()
						if err := s.refreshMaterializedViewConcurrent(b.ShortName, interval); err != nil {
							log.Printf("Refresh failed for %s: %v", b.ShortName, err)
						}
					}(board)
				}
				wg.Wait()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Storage) refreshMaterializedViewConcurrent(board domain.BoardShortName, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), interval*2)
	defer cancel()

	viewName := ViewTableName(board)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s", viewName),
	)
	if err != nil {
		return fmt.Errorf("concurrent refresh failed: %w", err)
	}
	return nil
}
