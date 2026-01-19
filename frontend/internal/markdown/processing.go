package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var messageLinkRegex = regexp.MustCompile(`&gt;&gt;(\d+)/(\d+)`)

// formatMessageLink generates HTML for a message link (used in markdown processing)
func formatMessageLink(board domain.BoardShortName, threadId domain.ThreadId, messageId domain.MsgId) string {
	return fmt.Sprintf(`<a href="/%s/%d#p%d" class="message-link" data-board="%[1]s" data-message-id="%[3]d" data-thread-id="%[2]d">>>%[2]d/%[3]d</a>`,
		board, threadId, messageId)
}

type TextProcessor struct {
	md goldmark.Markdown
}

func New() *TextProcessor {
	p := parser.NewParser(
		parser.WithBlockParsers(
			// util.Prioritized(parser.NewSetextHeadingParser(), 100),
			// util.Prioritized(parser.NewThematicBreakParser(), 200),
			// util.Prioritized(parser.NewListParser(), 300),
			// util.Prioritized(parser.NewListItemParser(), 400),
			// util.Prioritized(parser.NewCodeBlockParser(), 500),
			// util.Prioritized(parser.NewATXHeadingParser(), 600),
			util.Prioritized(parser.NewFencedCodeBlockParser(), 700),
			// util.Prioritized(parser.NewBlockquoteParser(), 800),
			// util.Prioritized(parser.NewHTMLBlockParser(), 900),
			util.Prioritized(NewLenientParagraphParser(), 1000),
		),
		parser.WithInlineParsers(
			util.Prioritized(parser.NewCodeSpanParser(), 100),
			// util.Prioritized(parser.NewLinkParser(), 200),
			// util.Prioritized(parser.NewAutoLinkParser(), 300),
			// util.Prioritized(parser.NewRawHTMLParser(), 400),
			util.Prioritized(parser.NewEmphasisParser(), 500),
		),
		// parser.WithBlockParsers(
		// 	util.Prioritized(parser.LinkReferenceParagraphTransformer, 100),
		// ),
	)

	md := goldmark.New(
		goldmark.WithParser(p),
		goldmark.WithRendererOptions(ghtml.WithUnsafe()),
		goldmark.WithExtensions(extension.Strikethrough),
	)
	return &TextProcessor{md: md}
}

func (tp *TextProcessor) ProcessMessage(message domain.Message) (string, domain.Replies, bool) {
	var err error
	// Render md and escape html
	message.Text, err = tp.renderText(message.Text)
	if err != nil {
		logger.Log.Error("rendering text", "error", err)
	}
	// Parse links
	processedText, matches := tp.processMessageLinks(message)
	// Sanitize html
	sanitizedText := tp.sanitizeText(processedText)
	// Check if message actually has payload
	hasPayload, err := tp.hasPayload(sanitizedText)
	if err != nil {
		logger.Log.Error("checking payload", "error", err)
	}

	return sanitizedText, matches, hasPayload
}

// processMessageLinks finds >>N/M patterns and converts them to internal links.
// It also returns a list of all matched strings found in the input.
func (tp *TextProcessor) processMessageLinks(message domain.Message) (string, domain.Replies) {
	var matches domain.Replies
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
		reply := domain.Reply{Board: message.Board, FromThreadId: message.ThreadId, ToThreadId: domain.ThreadId(threadId), From: message.Id, To: domain.MsgId(messageId)}
		linkHTML := formatMessageLink(message.Board, domain.ThreadId(threadId), domain.MsgId(messageId))
		// We dont want to add reply link twice
		if _, ok := seen[linkHTML]; !ok {
			seen[linkHTML] = struct{}{}
			matches = append(matches, &reply)
		}
		return linkHTML
	})

	return processedText, matches
}

func (tp *TextProcessor) renderText(text string) (string, error) {
	var buf bytes.Buffer
	if err := tp.md.Convert([]byte(text), &buf); err != nil {
		return text, err
	}
	unsafeHTML := buf.String()

	return strings.TrimSpace(unsafeHTML), nil
}

func (tp *TextProcessor) sanitizeText(text string) string {
	p := bluemonday.UGCPolicy()

	p.AllowAttrs("class").Matching(regexp.MustCompile("^message-link$")).OnElements("a")
	p.AllowAttrs("data-board", "data-message-id", "data-thread-id").OnElements("a")
	p.RequireNoFollowOnLinks(false)
	p.AllowRelativeURLs(true)

	safeHTML := p.Sanitize(text)
	return safeHTML
}

// hasPayload checks if an HTML string contains any text content that is not just whitespace.
func (tp *TextProcessor) hasPayload(htmlString string) (bool, error) {
	doc, err := html.Parse(strings.NewReader(htmlString))
	if err != nil {
		return false, err
	}

	var traverse func(*html.Node) bool
	traverse = func(n *html.Node) bool {
		if n.Type == html.TextNode {
			if strings.TrimSpace(n.Data) != "" {
				return true
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if traverse(c) {
				return true
			}
		}
		return false
	}

	return traverse(doc), nil
}
