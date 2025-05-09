package frontend_domain

import (
	"html/template"

	"github.com/itchan-dev/itchan/shared/domain"
)

type Message struct {
	domain.Message
	Text template.HTML
}
