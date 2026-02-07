package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

// linkRegex matches message links: &gt;&gt;threadId#msgId
var linkRegex = regexp.MustCompile(`&gt;&gt;(\d+)#(\d+)`)

// ConsumeResult indicates whether a block should continue or end
type ConsumeResult int

const (
	Continue ConsumeResult = iota
	End
)

// BlockRule defines how to parse a block-level element
type BlockRule struct {
	Name     string
	marker   string                                                               // prefix to match via trie
	Open     func(p *TextProcessor, firstLine string) string                      // called on first line
	Consume  func(p *TextProcessor, line string) (string, ConsumeResult)          // called on subsequent lines
	Close    func(p *TextProcessor, blockContent *strings.Builder) (string, bool) // called when block ends
	Validate func(firstLine string) bool                                          // if we need more complex matching validation than prefix
}

func (r *BlockRule) Marker() string {
	return r.marker
}

// InlineRule defines a simple marker-based inline formatting rule
type InlineRule struct {
	marker        string
	OpenTag       string
	CloseTag      string
	EscapeContent bool
}

func (r *InlineRule) Marker() string {
	return r.marker
}

// TextProcessor processes message text into safe HTML
type TextProcessor struct {
	cfg         *config.Public
	blockRules  []BlockRule
	blockTrie   *Trie
	inlineTrie  *Trie
	inlineRules []InlineRule

	// Per-message processing state
	currentMsg  *domain.Message
	replies     domain.Replies
	replyCount  int
	seenReplies map[string]struct{}
	hasPayload  bool
}

// New creates a new TextProcessor
func New(cfg *config.Public) *TextProcessor {
	p := &TextProcessor{
		cfg: cfg,
	}

	p.blockRules = []BlockRule{
		p.codeBlockRule(),
		p.greentextRule(),
	}

	p.inlineRules = []InlineRule{
		{marker: "**", OpenTag: "<strong>", CloseTag: "</strong>"},
		{marker: "~~", OpenTag: "<del>", CloseTag: "</del>"},
		{marker: "`", OpenTag: "<code>", CloseTag: "</code>", EscapeContent: true},
		{marker: "*", OpenTag: "<em>", CloseTag: "</em>"},
	}

	// Build trie for block rules
	p.blockTrie = newTrie()
	for _, rule := range p.blockRules {
		p.blockTrie.insert(&rule)
	}

	// Build trie for inline rules (longest match)
	p.inlineTrie = newTrie()
	for _, rule := range p.inlineRules {
		p.inlineTrie.insert(&rule)
	}

	return p
}

// codeBlockRule returns the rule for ``` code blocks
func (p *TextProcessor) codeBlockRule() BlockRule {
	return BlockRule{
		Name:   "codeblock",
		marker: "```",
		Open: func(p *TextProcessor, line string) string {
			return "<pre><code>"
		},
		Consume: func(p *TextProcessor, line string) (string, ConsumeResult) {
			if strings.HasPrefix(line, "```") {
				return "", End
			}
			if strings.TrimSpace(line) != "" {
				p.hasPayload = true
			}
			return escapeHTML(line) + "\n", Continue
		},
		Close: func(p *TextProcessor, blockContent *strings.Builder) (string, bool) {
			finalContent := strings.TrimSuffix(blockContent.String(), "\n")
			return finalContent + "</code></pre><br>", false
		},
		Validate: func(line string) bool {
			return true
		},
	}
}

// greentextRule returns the rule for > greentext
func (p *TextProcessor) greentextRule() BlockRule {
	return BlockRule{
		Name:   "greentext",
		marker: ">",
		Open: func(p *TextProcessor, line string) string {
			return `<span class="greentext">` + p.parseInline(line)
		},
		Consume: func(p *TextProcessor, line string) (string, ConsumeResult) {
			// End block if: empty line, doesn't start with >, or is a message link (>>X but not >>>)
			if len(line) == 0 || line[0] != '>' ||
				(len(line) > 2 && line[1] == '>' && line[2] != '>') {
				return "", End
			}
			return "<br>" + p.parseInline(line), Continue
		},
		Close: func(p *TextProcessor, blockContent *strings.Builder) (string, bool) {
			return blockContent.String() + "</span><br>", true
		},
		Validate: func(line string) bool {
			if len(line) > 2 && line[1] == '>' && line[2] != '>' {
				return false
			}
			return true
		},
	}
}

// ProcessMessage converts raw text to safe HTML
func (p *TextProcessor) ProcessMessage(msg domain.Message) (string, domain.Replies, bool, error) {
	p.currentMsg = &msg
	p.replies = nil
	p.replyCount = 0
	p.seenReplies = make(map[string]struct{})
	p.hasPayload = false

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return "", nil, false, nil
	}

	lines := strings.Split(text, "\n")
	var result strings.Builder
	result.Grow(len(text) * 2)

	var activeRule *BlockRule
	var blockContent strings.Builder
	emptyLineCount := 0

	for _, line := range lines {
		// Inside an active block
		if activeRule != nil {
			content, status := activeRule.Consume(p, line)
			if status == End {
				// If block ended, finalize block content and close
				finalContent, continueProcessing := activeRule.Close(p, &blockContent)
				result.WriteString(finalContent)
				activeRule = nil
				blockContent.Reset()
				if !continueProcessing {
					continue
				}
			} else {
				// Otherwise write consumed line
				blockContent.WriteString(content)
				continue
			}
		}

		// Empty line
		if strings.TrimSpace(line) == "" {
			if emptyLineCount < 2 { // limit on consecutive empty lines
				result.WriteString("<br>")
			}
			emptyLineCount++
			continue
		}
		emptyLineCount = 0

		if matches := p.blockTrie.Match(line, 0); len(matches) > 0 {
			longestMatch := matches[len(matches)-1]
			rule, ok := longestMatch.Rule.(*BlockRule)
			if !ok {
				return "", domain.Replies{}, false, fmt.Errorf("BlockRule type assertions failed")
			}
			if rule.Validate(line) {
				blockContent.WriteString(rule.Open(p, line))
				activeRule = rule
				continue
			}
		}

		// Parse inline content if this is not a block or empty line
		result.WriteString(p.parseInline(line) + "<br>")
	}

	// Handle unclosed block at EOF (close it)
	if activeRule != nil {
		finalContent, _ := activeRule.Close(p, &blockContent)
		result.WriteString(finalContent)
	}
	resultStr := strings.TrimSuffix(result.String(), "<br>")

	// Phase 2: Process message links (skip inside code blocks)
	html := p.processLinks(resultStr)

	return html, p.replies, p.hasPayload, nil
}

// parseInline handles inline formatting using a stack-based approach:
//   - When a marker is found, push current content and marker to stack
//   - When a closing marker is found, pop back to opening marker and wrap content
//   - Unclosed markers are output literally (imageboard-style)
func (p *TextProcessor) parseInline(line string) string {
	var stack []string
	var current strings.Builder
	var pos int
	markerStartPos := make(map[string]int)

	for pos < len(line) {
		if matches := p.inlineTrie.Match(line, pos); len(matches) > 0 {
			// if we matched delimiter (marker), add current string to stack and reset
			stack = append(stack, current.String())
			current.Reset()
			rule, ok := matches[len(matches)-1].Rule.(*InlineRule)
			if !ok {
				// shouldn't happen
				return ""
			}
			matchLen := matches[len(matches)-1].Len
			// If we already have this rule in our stack
			if startPos, ok := markerStartPos[rule.Marker()]; ok {
				var ruleContent string
				offset := 1
				// Find the marker in stack and calculate offset
				for stack[len(stack)-offset] != rule.Marker() {
					elem := stack[len(stack)-offset]
					// Clean up markerStartPos for any markers we're removing
					delete(markerStartPos, elem)
					offset++
				}
				if rule.EscapeContent {
					// if we need to escape content, just take original line, dont use stack
					ruleContent = escapeHTML(line[startPos+matchLen : pos])
				} else {
					// if we dont need to escape content, take stack tail (before marker) and make it content
					ruleContent = strings.Join(stack[len(stack)-offset+1:], "")
				}
				// Reject empty/whitespace-only content - treat markers as literal
				if strings.TrimSpace(ruleContent) == "" {
					delete(markerStartPos, rule.Marker())
					stack = append(stack, rule.marker)
					p.hasPayload = true // markers are visible text
				} else {
					// concat stack tail before marker and add tags and make it new last stack element (processed)
					stack = stack[:len(stack)-offset]
					stack = append(stack, rule.OpenTag+ruleContent+rule.CloseTag)
					delete(markerStartPos, rule.Marker())
				}
			} else {
				// if this marker is not opened
				stack = append(stack, rule.marker)
				markerStartPos[rule.Marker()] = pos
				p.hasPayload = true // markers are visible text
			}
			// skip mark and go further
			for range matchLen {
				pos++
			}
			continue
		}

		// No match - escape and output character
		c := line[pos]
		p.escapeCharPayload(&current, c)
		pos++
	}

	stack = append(stack, current.String())
	return strings.Join(stack, "")
}

// processLinks finds &gt;&gt;threadId#msgId patterns and converts to anchor tags
// Skips replacements inside <code>...</code> tags
func (p *TextProcessor) processLinks(html string) string {
	// Split by code tags to avoid processing links inside code
	var result strings.Builder
	result.Grow(len(html))

	pos := 0
	for pos < len(html) {
		// Look for <code> or <pre><code>
		codeStart := strings.Index(html[pos:], "<code>")
		if codeStart == -1 {
			// No more code blocks, process the rest
			result.WriteString(p.replaceLinks(html[pos:]))
			break
		}
		codeStart += pos

		// Process content before code block
		result.WriteString(p.replaceLinks(html[pos:codeStart]))

		// Find closing </code>
		codeEnd := strings.Index(html[codeStart:], "</code>")
		if codeEnd == -1 {
			// Unclosed code block, output rest as-is
			result.WriteString(html[codeStart:])
			break
		}
		codeEnd += codeStart + len("</code>")

		// Output code block unchanged
		result.WriteString(html[codeStart:codeEnd])
		pos = codeEnd
	}

	return result.String()
}

// replaceLinks replaces &gt;&gt;threadId#msgId with anchor tags
func (p *TextProcessor) replaceLinks(text string) string {
	return linkRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatch := linkRegex.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}

		threadId, err1 := strconv.ParseInt(submatch[1], 10, 64)
		msgId, err2 := strconv.ParseInt(submatch[2], 10, 64)
		if err1 != nil || err2 != nil {
			return match
		}

		// Build link key for deduplication
		key := fmt.Sprintf("%d#%d", threadId, msgId)

		// Check if already seen (still render link, just don't add to replies)
		if _, exists := p.seenReplies[key]; !exists {
			// Check reply limit
			if p.replyCount < p.cfg.MaxRepliesPerMessage {
				p.seenReplies[key] = struct{}{}
				p.replyCount++

				reply := &domain.Reply{
					Board:        p.currentMsg.Board,
					FromThreadId: p.currentMsg.ThreadId,
					ToThreadId:   domain.ThreadId(threadId),
					From:         p.currentMsg.Id,
					To:           domain.MsgId(msgId),
				}
				p.replies = append(p.replies, reply)
			}
		}

		return p.formatMessageLink(domain.ThreadId(threadId), domain.MsgId(msgId))
	})
}

// formatMessageLink generates HTML for a message link
func (p *TextProcessor) formatMessageLink(threadId domain.ThreadId, msgId domain.MsgId) string {
	board := escapeHTML(string(p.currentMsg.Board))
	return fmt.Sprintf(`<a href="/%s/%d#p%d" class="message-link message-link-preview" data-board="%s" data-message-id="%d" data-thread-id="%d">&gt;&gt;%d#%d</a>`,
		p.currentMsg.Board, threadId, msgId, board, msgId, threadId, threadId, msgId)
}

// escapeChar escapes a single character for HTML output
func escapeChar(result *strings.Builder, c byte) {
	switch c {
	case '<':
		result.WriteString("&lt;")
	case '>':
		result.WriteString("&gt;")
	case '&':
		result.WriteString("&amp;")
	case '"':
		result.WriteString("&quot;")
	default:
		result.WriteByte(c)
	}
}

// escapeCharPayload escapes a character and tracks non-whitespace for hasPayload
func (p *TextProcessor) escapeCharPayload(result *strings.Builder, c byte) {
	if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
		p.hasPayload = true
	}
	escapeChar(result, c)
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		escapeChar(&b, s[i])
	}
	return b.String()
}
