package frontend_domain

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type Board struct {
	domain.Board
	Threads []*Thread
}
