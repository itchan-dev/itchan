package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

type IndexPageData struct {
	PublicBoards    []domain.Board
	CorporateBoards []domain.Board
}
