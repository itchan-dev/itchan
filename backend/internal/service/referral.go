package service

import (
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
)

type ReferralService interface {
	RecordAction(source, action, ip string) error
	GetStats() ([]domain.ReferralActionStats, error)
}

type ReferralStorage interface {
	SaveReferralAction(source, action, ip string) error
	GetReferralActionStats() ([]domain.ReferralActionStats, error)
}

type Referral struct {
	storage ReferralStorage
}

func NewReferral(storage ReferralStorage) *Referral {
	return &Referral{storage: storage}
}

func (s *Referral) RecordAction(source, action, ip string) error {
	source = sanitizeSource(source)
	return s.storage.SaveReferralAction(source, action, ip)
}

func (s *Referral) GetStats() ([]domain.ReferralActionStats, error) {
	return s.storage.GetReferralActionStats()
}

func sanitizeSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if len(source) > 100 {
		source = source[:100]
	}
	return source
}
