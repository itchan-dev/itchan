package pg

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
)

// StartPeriodicViewRefresh initiates a background goroutine that periodically refreshes
// materialized views for active boards. This ensures board preview data stays up-to-date
// without blocking user requests.
//
// The refresh process:
//  1. Runs on a ticker interval (configured via cfg.Public.BoardPreviewRefreshInterval)
//  2. Identifies boards with recent activity within the interval period
//  3. Concurrently refreshes each active board's materialized view
//  4. Stops gracefully when the context is canceled
//
// This is called once during application initialization from New().
func (s *Storage) StartPeriodicViewRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				boards, err := s.GetActiveBoards(interval)
				if err != nil {
					logger.Log.Error("failed to fetch active boards", "error", err)
					continue
				}
				var wg sync.WaitGroup
				for _, board := range boards {
					wg.Add(1)
					go func(b domain.Board) {
						defer wg.Done()
						if err := s.refreshMaterializedViewConcurrent(b.ShortName, interval); err != nil {
							logger.Log.Error("materialized view refresh failed",
								"board", b.ShortName,
								"error", err)
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

// refreshMaterializedViewConcurrent performs a concurrent refresh of a board's materialized view.
// It uses REFRESH MATERIALIZED VIEW CONCURRENTLY, which allows reads to continue during the
// refresh operation, making it suitable for production use.
//
// The timeout is set to 2x the refresh interval to allow adequate time for the operation
// while preventing indefinite hangs.
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

// refreshMaterializedView performs a non-concurrent refresh of a board's materialized view.
//
// Design Note: Production vs Test Refresh Behavior
// ------------------------------------------------
// This method uses REFRESH MATERIALIZED VIEW without the CONCURRENTLY option, which has
// important implications for both production and testing scenarios:
//
// 1. Transaction Visibility:
//    - Non-concurrent refresh CAN see uncommitted data within the same transaction
//    - Concurrent refresh (CONCURRENTLY) only sees committed data
//    This makes non-concurrent refresh suitable for transactional tests where data hasn't
//    been committed yet.
//
// 2. Locking Behavior:
//    - Non-concurrent refresh takes an AccessExclusiveLock, preventing reads during refresh
//    - Concurrent refresh allows reads to continue during the refresh operation
//    This makes concurrent refresh preferable for production use.
//
// Usage:
//    - Production: Use refreshMaterializedViewConcurrent() to avoid blocking readers
//    - Tests: Use this method within transactions to test with uncommitted data
//
// Implementation:
//    This method accepts a Querier interface, allowing it to operate within a transaction
//    context when needed (for tests) or directly against the database (if needed elsewhere).
func (s *Storage) refreshMaterializedView(q Querier, board domain.BoardShortName) error {
	viewName := ViewTableName(board)
	_, err := q.Exec(fmt.Sprintf("REFRESH MATERIALIZED VIEW %s", viewName))
	if err != nil {
		return fmt.Errorf("refresh failed: %w", err)
	}
	return nil
}
