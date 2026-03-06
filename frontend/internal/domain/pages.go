package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

type BoardWithAccess struct {
	domain.Board
	Accessible bool
}

type IndexPageData struct {
	PublicBoards    []domain.Board
	CorporateBoards []BoardWithAccess
}

type BlacklistedUsers struct {
	Users []domain.BlacklistEntry
	Page  int
}

type AdminPageData struct {
	Blacklisted BlacklistedUsers
	RefStats    []domain.ReferralActionStats
}
