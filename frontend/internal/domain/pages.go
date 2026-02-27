package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

type IndexPageData struct {
	PublicBoards    []domain.Board
	CorporateBoards []domain.Board
}

type BlacklistedUsers struct {
	Users []domain.BlacklistEntry
	Page  int
}

type AdminPageData struct {
	Blacklisted BlacklistedUsers
	RefStats    []domain.ReferralStats
}
