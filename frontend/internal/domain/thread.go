package frontend_domain

import (
	"github.com/itchan-dev/itchan/shared/domain"
)

type Thread struct {
	domain.ThreadMetadata
	Messages []Message
}
