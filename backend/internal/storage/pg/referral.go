package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

func (s *Storage) SaveReferralVisit(source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO referral_visits(source) VALUES($1)", source)
		if err != nil {
			return fmt.Errorf("failed to insert referral visit: %w", err)
		}
		return nil
	})
}

func (s *Storage) SaveReferralRegistration(userId domain.UserId, source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"INSERT INTO referral_registrations(user_id, source) VALUES($1, $2) ON CONFLICT (user_id) DO NOTHING",
			userId, source,
		)
		if err != nil {
			return fmt.Errorf("failed to insert referral registration: %w", err)
		}
		return nil
	})
}

func (s *Storage) GetReferralStats() ([]domain.ReferralStats, error) {
	rows, err := s.db.Query(`
		SELECT
			COALESCE(v.source, r.source) AS source,
			COALESCE(v.visit_count, 0) AS visit_count,
			COALESCE(r.register_count, 0) AS register_count
		FROM
			(SELECT source, COUNT(*) AS visit_count FROM referral_visits GROUP BY source) v
		FULL OUTER JOIN
			(SELECT source, COUNT(*) AS register_count FROM referral_registrations GROUP BY source) r
		ON v.source = r.source
		ORDER BY visit_count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query referral stats: %w", err)
	}
	defer rows.Close()

	var stats []domain.ReferralStats
	for rows.Next() {
		var s domain.ReferralStats
		if err := rows.Scan(&s.Source, &s.VisitCount, &s.RegisterCount); err != nil {
			return nil, fmt.Errorf("failed to scan referral stats: %w", err)
		}
		if s.VisitCount > 0 {
			s.ConversionRate = float64(s.RegisterCount) * 100.0 / float64(s.VisitCount)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}
