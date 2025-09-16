package frontend_domain

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type Thread struct {
	domain.Thread
	Messages []*Message
}
