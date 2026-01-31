package markdown

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
	"github.com/itchan-dev/itchan/shared/logger"
	"github.com/itchan-dev/itchan/shared/utils"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"

	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var messageLinkRegex = regexp.MustCompile(`&gt;&gt;(\d+)#(\d+)`)
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// Cached sanitizer policy - compiled once at startup
var sanitizerPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Matching(regexp.MustCompile("^message-link( message-link-preview)?$")).OnElements("a")
	p.AllowAttrs("data-board", "data-message-id", "data-thread-id").OnElements("a")
	p.RequireNoFollowOnLinks(false)
	p.AllowRelativeURLs(true)
	// Allow greentext spans
	p.AllowAttrs("class").Matching(regexp.MustCompile("^greentext$")).OnElements("span")
	return p
}()

// formatMessageLink generates HTML for an inline message link with page calculation.
// The page param is calculated server-side and stored in the HTML.
func (tp *TextProcessor) formatMessageLink(board domain.BoardShortName, threadId domain.ThreadId, messageId domain.MsgId) string {
	page := utils.CalculatePage(int(messageId), tp.cfg.MessagesPerThreadPage)

	pageParam := ""
	if page > 1 {
		pageParam = fmt.Sprintf("?page=%d", page)
	}

	return fmt.Sprintf(`<a href="/%s/%d%s#p%d" class="message-link message-link-preview" data-board="%s" data-message-id="%d" data-thread-id="%d">&gt;&gt;%d#%d</a>`,
		board, threadId, pageParam, messageId, board, messageId, threadId, threadId, messageId)
}

type TextProcessor struct {
	md  goldmark.Markdown
	cfg *config.Public
}

func New(cfg *config.Public) *TextProcessor {
	p := parser.NewParser(
		parser.WithBlockParsers(
			// util.Prioritized(parser.NewSetextHeadingParser(), 100),
			// util.Prioritized(parser.NewThematicBreakParser(), 200),
			// util.Prioritized(parser.NewListParser(), 300),
			// util.Prioritized(parser.NewListItemParser(), 400),
			// util.Prioritized(parser.NewCodeBlockParser(), 500),
			// util.Prioritized(parser.NewATXHeadingParser(), 600),
			util.Prioritized(NewGreentextParser(), 650),
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
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(),
		),
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewGreentextHTMLRenderer(), 100),
			),
		),
	)
	return &TextProcessor{
		md:  md,
		cfg: cfg,
	}
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

// processMessageLinks finds >>N#M patterns and converts them to internal links.
// It also returns a list of all matched strings found in the input.
// The number of unique reply links is limited by MaxRepliesPerMessage config.
// Uses early termination to avoid processing matches after limit is reached.
func (tp *TextProcessor) processMessageLinks(message domain.Message) (string, domain.Replies) {
	var matches domain.Replies
	seen := make(map[string]struct{})
	replyCount := 0

	text := message.Text
	var result strings.Builder
	result.Grow(len(text)) // Pre-allocate to avoid reallocations
	lastEnd := 0

	for {
		// Find next match starting from lastEnd
		loc := messageLinkRegex.FindStringSubmatchIndex(text[lastEnd:])
		if loc == nil {
			break
		}

		// Adjust indices relative to full string
		matchStart := lastEnd + loc[0]
		matchEnd := lastEnd + loc[1]

		// Write text before this match
		result.WriteString(text[lastEnd:matchStart])

		// Parse thread and message IDs from capture groups
		threadIdStr := text[lastEnd+loc[2] : lastEnd+loc[3]]
		msgIdStr := text[lastEnd+loc[4] : lastEnd+loc[5]]

		threadId, err1 := strconv.ParseInt(threadIdStr, 10, 64)
		messageId, err2 := strconv.ParseInt(msgIdStr, 10, 64)

		if err1 != nil || err2 != nil {
			// Invalid numbers, keep as raw text
			result.WriteString(text[matchStart:matchEnd])
			lastEnd = matchEnd
			continue
		}

		linkHTML := tp.formatMessageLink(message.Board, domain.ThreadId(threadId), domain.MsgId(messageId))

		if _, ok := seen[linkHTML]; !ok {
			// New unique reply
			if replyCount >= tp.cfg.MaxRepliesPerMessage {
				// Limit reached - append remaining text unchanged and exit
				result.WriteString(text[matchStart:])
				return result.String(), matches
			}
			seen[linkHTML] = struct{}{}
			replyCount++
			reply := domain.Reply{
				Board:        message.Board,
				FromThreadId: message.ThreadId,
				ToThreadId:   domain.ThreadId(threadId),
				From:         message.Id,
				To:           domain.MsgId(messageId),
			}
			matches = append(matches, &reply)
		}

		result.WriteString(linkHTML)
		lastEnd = matchEnd
	}

	// Append any remaining text after last match
	result.WriteString(text[lastEnd:])
	return result.String(), matches
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
	return sanitizerPolicy.Sanitize(text)
}

// hasPayload checks if an HTML string contains any text content that is not just whitespace.
// Uses fast regex-based approach: strip all HTML tags and check if any text remains.
// This is much faster than full HTML parsing since we already sanitized the HTML with Bluemonday.
func (tp *TextProcessor) hasPayload(htmlString string) (bool, error) {
	// Strip all HTML tags (including <br>, <span>, <p>, etc.)
	textOnly := htmlTagRegex.ReplaceAllString(htmlString, "")

	// Decode HTML entities (&gt;, &lt;, etc.) and check if any text remains
	textOnly = html.UnescapeString(textOnly)

	// Check if there's any non-whitespace text
	return strings.TrimSpace(textOnly) != "", nil
}
