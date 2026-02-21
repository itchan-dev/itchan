package frontend_domain

import "github.com/itchan-dev/itchan/shared/domain"

type IndexPageData struct {
	PublicBoards    []domain.Board
	CorporateBoards []domain.Board
}

type BoardPageData struct {
	Board       *Board
	CurrentPage int
}

type ThreadPageData struct {
	Thread *Thread
}

type AccountPageData struct {
	ActivityMessages []*Message
}
