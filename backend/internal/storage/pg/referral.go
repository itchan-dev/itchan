package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

func (s *Storage) SaveReferralAction(source, action, ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"INSERT INTO referral_actions(source, action, ip) VALUES($1, $2, $3::inet) ON CONFLICT(ip, source, action) DO NOTHING",
			source, action, ip,
		)
		if err != nil {
			return fmt.Errorf("failed to insert referral action: %w", err)
		}
		return nil
	})
}

func (s *Storage) GetReferralActionStats() ([]domain.ReferralActionStats, error) {
	rows, err := s.db.Query(`
		SELECT source, action, COUNT(*) AS count
		FROM referral_actions
		GROUP BY source, action
		ORDER BY source, action
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query referral action stats: %w", err)
	}
	defer rows.Close()

	var stats []domain.ReferralActionStats
	for rows.Next() {
		var s domain.ReferralActionStats
		if err := rows.Scan(&s.Source, &s.Action, &s.Count); err != nil {
			return nil, fmt.Errorf("failed to scan referral action stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}
