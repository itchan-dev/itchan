package frontend_domain

// PostData is the typed data for the "post" template partial.
type PostData struct {
	Message *Message
	Common  *CommonTemplateData
}
