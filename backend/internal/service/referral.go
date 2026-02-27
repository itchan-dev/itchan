package service

import (
	"strings"

	"github.com/itchan-dev/itchan/shared/domain"
)

type ReferralService interface {
	RecordVisit(source string) error
	RecordRegistration(userId domain.UserId, source string) error
	GetStats() ([]domain.ReferralStats, error)
}

type ReferralStorage interface {
	SaveReferralVisit(source string) error
	SaveReferralRegistration(userId domain.UserId, source string) error
	GetReferralStats() ([]domain.ReferralStats, error)
}

type Referral struct {
	storage ReferralStorage
}

func NewReferral(storage ReferralStorage) *Referral {
	return &Referral{storage: storage}
}

func (s *Referral) RecordVisit(source string) error {
	source = sanitizeSource(source)
	return s.storage.SaveReferralVisit(source)
}

func (s *Referral) RecordRegistration(userId domain.UserId, source string) error {
	source = sanitizeSource(source)
	return s.storage.SaveReferralRegistration(userId, source)
}

func (s *Referral) GetStats() ([]domain.ReferralStats, error) {
	return s.storage.GetReferralStats()
}

func sanitizeSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if len(source) > 100 {
		source = source[:100]
	}
	return source
}
