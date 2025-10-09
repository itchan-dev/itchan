package markdown

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	frontend_domain "github.com/itchan-dev/itchan/frontend/internal/domain"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

var messageLinkRegex = regexp.MustCompile(`>>(\d+)/(\d+)`)

// var escapedMessageLinkRegex = regexp.MustCompile(`&gt;&gt;(\d+)/(\d+)`)

func ProcessMessage(message domain.Message) (string, frontend_domain.Replies) {
	processedText, matches := processMessageLinks(message)
	renderedText, _ := renderText(processedText)
	sanitizedText := sanitizeText(renderedText)
	return sanitizedText, matches
}

// processMessageLinks finds >>N/M patterns and converts them to internal links.
// It also returns a list of all matched strings found in the input.
func processMessageLinks(message domain.Message) (string, frontend_domain.Replies) {
	var matches frontend_domain.Replies
	seen := make(map[string]struct{})

	processedText := messageLinkRegex.ReplaceAllStringFunc(message.Text, func(match string) string {
		// Extract the capture groups from the current match
		submatch := messageLinkRegex.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match // shouldn't happen due to prior match
		}
		threadId, err := strconv.ParseInt(submatch[1], 10, 64)
		if err != nil {
			return match
		}
		messageId, err := strconv.ParseInt(submatch[2], 10, 64)
		if err != nil {
			return match
		}
		reply := frontend_domain.Reply{Reply: domain.Reply{Board: message.Board, FromThreadId: message.ThreadId, ToThreadId: threadId, From: message.Id, To: messageId}}
		linkTo := reply.LinkTo()
		// We dont want to add reply link twice
		if _, ok := seen[linkTo]; !ok {
			seen[linkTo] = struct{}{}
			matches = append(matches, &reply)
		}
		return reply.LinkTo()
	})

	return processedText, matches
}

func renderText(text string) (string, error) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(text), &buf); err != nil {
		return text, err
	}
	unsafeHTML := buf.String()

	return strings.TrimSpace(unsafeHTML), nil
}

func sanitizeText(text string) string {
	p := bluemonday.UGCPolicy()

	p.AllowAttrs("class").Matching(regexp.MustCompile("^message-link$")).OnElements("a")
	p.AllowAttrs("data-board", "data-message-id", "data-thread-id").OnElements("a")
	p.RequireNoFollowOnLinks(false)
	p.AllowRelativeURLs(true)

	safeHTML := p.Sanitize(text)
	return safeHTML
}
