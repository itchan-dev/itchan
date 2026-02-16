package markdown

import (
	"strings"
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

func TestRender(t *testing.T) {
	cfg := &config.Public{
		MaxRepliesPerMessage: 10,
	}
	tp := New(cfg)

	tests := []struct {
		name       string
		input      string
		expected   string
		hasPayload bool
	}{
		// Basic formatting
		{
			name:       "normal text",
			input:      "hello world",
			expected:   "hello world",
			hasPayload: true,
		},
		{
			name:       "bold text",
			input:      "**hello**",
			expected:   "<strong>hello</strong>",
			hasPayload: true,
		},
		{
			name:       "italic text",
			input:      "*hello*",
			expected:   "<em>hello</em>",
			hasPayload: true,
		},
		{
			name:       "strikethrough text",
			input:      "~~hello~~",
			expected:   "<del>hello</del>",
			hasPayload: true,
		},
		{
			name:       "spoiler text",
			input:      "||spoiler||",
			expected:   `<span class="spoiler">spoiler</span>`,
			hasPayload: true,
		},
		{
			name:       "inline code",
			input:      "`code`",
			expected:   "<code>code</code>",
			hasPayload: true,
		},

		// Greentext
		{
			name:       "single line greentext",
			input:      ">implying",
			expected:   `<span class="greentext">&gt;implying</span>`,
			hasPayload: true,
		},
		{
			name:       "multi line greentext",
			input:      ">be me\n>using imageboard",
			expected:   `<span class="greentext">&gt;be me<br>&gt;using imageboard</span>`,
			hasPayload: true,
		},
		{
			name:       "greentext with blank line separator",
			input:      ">first line\n\n>second line",
			expected:   `<span class="greentext">&gt;first line</span><br><br><span class="greentext">&gt;second line</span>`,
			hasPayload: true,
		},
		{
			name:       "greentext and normal text",
			input:      ">greentext line\nnormal text",
			expected:   `<span class="greentext">&gt;greentext line</span><br>normal text`,
			hasPayload: true,
		},
		{
			name:       "empty greentext",
			input:      ">",
			expected:   `<span class="greentext">&gt;</span>`,
			hasPayload: true, // ">" is visible as "&gt;"
		},
		{
			name:       "greentext plus link",
			input:      ">>>123#456",
			expected:   `<span class="greentext">&gt;<a href="/test/123#p456" class="message-link message-link-preview" data-board="test" data-message-id="456" data-thread-id="123">&gt;&gt;123#456</a></span>`,
			hasPayload: true,
		},

		// Code blocks
		{
			name:       "code block",
			input:      "```\ncode here\n```",
			expected:   "<pre><code>code here</code></pre>",
			hasPayload: true,
		},
		{
			name:       "code block with special chars",
			input:      "```\n<html>&test\n```",
			expected:   "<pre><code>&lt;html&gt;&amp;test</code></pre>",
			hasPayload: true,
		},
		{
			name:       "empty code block",
			input:      "```\n```",
			expected:   "<pre><code></code></pre>",
			hasPayload: false,
		},
		{
			name:       "code block with whitespace line",
			input:      "```\n   \n```",
			expected:   "<pre><code>   </code></pre>",
			hasPayload: false,
		},

		// Unclosed blocks
		{
			name:       "unclosed code block with content",
			input:      "```\nabc",
			expected:   "<pre><code>abc</code></pre>",
			hasPayload: true,
		},
		{
			name:       "unclosed code block empty",
			input:      "```",
			expected:   "<pre><code></code></pre>",
			hasPayload: false,
		},
		{
			name:       "unclosed greentext",
			input:      ">abc",
			expected:   `<span class="greentext">&gt;abc</span>`,
			hasPayload: true,
		},
		{
			name:       "unclosed greentext multiline",
			input:      ">line1\n>line2\n>line3",
			expected:   `<span class="greentext">&gt;line1<br>&gt;line2<br>&gt;line3</span>`,
			hasPayload: true,
		},

		// Unclosed inline formatting
		{
			name:       "unclosed bold",
			input:      "*abc",
			expected:   "*abc",
			hasPayload: true,
		},
		{
			name:       "unclosed bold at end",
			input:      "text *abc",
			expected:   "text *abc",
			hasPayload: true,
		},
		{
			name:       "unclosed strikethrough",
			input:      "~~abc",
			expected:   "~~abc",
			hasPayload: true,
		},
		{
			name:       "unclosed spoiler",
			input:      "||abc",
			expected:   "||abc",
			hasPayload: true,
		},
		{
			name:       "unclosed double bold",
			input:      "**abc",
			expected:   "**abc",
			hasPayload: true,
		},
		{
			name:       "unclosed inline code",
			input:      "`abc",
			expected:   "`abc",
			hasPayload: true,
		},
		{
			name:       "mixed unclosed inline",
			input:      "*a~~bc~~",
			expected:   "*a<del>bc</del>",
			hasPayload: true,
		},
		{
			name:       "nested unclosed inline",
			input:      "**text *unclosed",
			expected:   "**text *unclosed",
			hasPayload: true,
		},

		// HTML escaping
		{
			name:       "escape HTML entities",
			input:      "<script>alert('xss')</script>",
			expected:   "&lt;script&gt;alert('xss')&lt;/script&gt;",
			hasPayload: true,
		},
		{
			name:       "escape ampersand",
			input:      "A & B",
			expected:   "A &amp; B",
			hasPayload: true,
		},
		{
			name:       "escape quotes",
			input:      `He said "hello"`,
			expected:   `He said &quot;hello&quot;`,
			hasPayload: true,
		},

		// Empty/whitespace - imageboard style (trimmed)
		{
			name:       "empty string",
			input:      "",
			expected:   "",
			hasPayload: false,
		},
		{
			name:       "only whitespace",
			input:      "   \n\n   ",
			expected:   "",
			hasPayload: false,
		},
		{
			name:       "leading empty lines trimmed",
			input:      "\n\nhello",
			expected:   "hello",
			hasPayload: true,
		},
		{
			name:       "trailing empty lines trimmed",
			input:      "hello\n\n",
			expected:   "hello",
			hasPayload: true,
		},

		// Message links
		{
			name:       "message link renders as link",
			input:      ">>123#456",
			expected:   `<a href="/test/123#p456" class="message-link message-link-preview" data-board="test" data-message-id="456" data-thread-id="123">&gt;&gt;123#456</a>`,
			hasPayload: true,
		},

		// Edge cases - empty/whitespace formatting (literal output)
		{
			name:       "only bold formatting markers",
			input:      "**  **",
			expected:   "**  **",
			hasPayload: true,
		},
		{
			name:       "only strikethrough markers",
			input:      "~~  ~~",
			expected:   "~~  ~~",
			hasPayload: true,
		},
		{
			name:       "empty inline code markers",
			input:      "``",
			expected:   "``",
			hasPayload: true,
		},
		{
			name:       "completely empty bold",
			input:      "****",
			expected:   "****",
			hasPayload: true,
		},
		{
			name:       "completely empty spoiler",
			input:      "||||",
			expected:   "||||",
			hasPayload: true,
		},
		{
			name:       "whitespace-only spoiler",
			input:      "||  ||",
			expected:   "||  ||",
			hasPayload: true,
		},

		// Multiple blocks
		{
			name:       "multiple empty code blocks",
			input:      "```\n```\n\n```\n```",
			expected:   "<pre><code></code></pre><br><br><pre><code></code></pre>",
			hasPayload: false,
		},
		{
			name:       "code block then text",
			input:      "```\n```\n\nhello",
			expected:   "<pre><code></code></pre><br><br>hello",
			hasPayload: true,
		},

		// Nested formatting is supported
		{
			name:       "bold inside italic",
			input:      "*italic **bold** italic*",
			expected:   "<em>italic <strong>bold</strong> italic</em>",
			hasPayload: true,
		},
		{
			name:       "italic inside bold",
			input:      "**bold *italic* bold**",
			expected:   "<strong>bold <em>italic</em> bold</strong>",
			hasPayload: true,
		},
		{
			name:       "strikethrough inside bold",
			input:      "**bold ~~deleted~~ bold**",
			expected:   "<strong>bold <del>deleted</del> bold</strong>",
			hasPayload: true,
		},
		{
			name:       "code inside bold",
			input:      "**bold `code` bold**",
			expected:   "<strong>bold <code>code</code> bold</strong>",
			hasPayload: true,
		},
		{
			name:       "spoiler inside bold",
			input:      "**bold ||secret|| bold**",
			expected:   `<strong>bold <span class="spoiler">secret</span> bold</strong>`,
			hasPayload: true,
		},
		{
			name:       "bold inside spoiler",
			input:      "||**bold secret**||",
			expected:   `<span class="spoiler"><strong>bold secret</strong></span>`,
			hasPayload: true,
		},

		// Inline code special handling
		{
			name:       "inline code with html",
			input:      "`<div>html</div>`",
			expected:   "<code>&lt;div&gt;html&lt;/div&gt;</code>",
			hasPayload: true,
		},
		{
			name:       "inline code with formatting markers",
			input:      "`**not bold**`",
			expected:   "<code>**not bold**</code>",
			hasPayload: true,
		},

		// Formatting boundaries
		{
			name:       "bold mid-word",
			input:      "un**believ**able",
			expected:   "un<strong>believ</strong>able",
			hasPayload: true,
		},
		{
			name:       "adjacent formatting",
			input:      "**bold***italic*",
			expected:   "<strong>bold</strong><em>italic</em>",
			hasPayload: true,
		},

		// Multiline in code blocks
		{
			name:       "code block multiline",
			input:      "```\nline1\nline2\nline3\n```",
			expected:   "<pre><code>line1\nline2\nline3</code></pre>",
			hasPayload: true,
		},
		{
			name:       "code block preserves indentation",
			input:      "```\n  indented\n    more\n```",
			expected:   "<pre><code>  indented\n    more</code></pre>",
			hasPayload: true,
		},

		// Text before and after blocks
		{
			name:       "text before code block",
			input:      "hello\n```\ncode\n```",
			expected:   "hello<br><pre><code>code</code></pre>",
			hasPayload: true,
		},
		{
			name:       "text after code block",
			input:      "```\ncode\n```\nhello",
			expected:   "<pre><code>code</code></pre><br>hello",
			hasPayload: true,
		},

		// Multiple newlines (imageboard-style: max 3 <br>)
		{
			name:       "three newlines collapsed",
			input:      "hello\n\n\nworld",
			expected:   "hello<br><br><br>world",
			hasPayload: true,
		},
		{
			name:       "4 newlines collapsed",
			input:      "hello\n\n\n\nworld",
			expected:   "hello<br><br><br>world",
			hasPayload: true,
		},
		{
			name:       "two newlines preserved",
			input:      "hello\n\nworld",
			expected:   "hello<br><br>world",
			hasPayload: true,
		},
		{
			name:       "text newline text",
			input:      "line1\nline2",
			expected:   "line1<br>line2",
			hasPayload: true,
		},
		{
			name:       "double newline paragraph break",
			input:      "line1\n\nline2",
			expected:   "line1<br><br>line2",
			hasPayload: true,
		},

		// Greentext edge cases
		{
			name:       "greentext with formatting",
			input:      ">**bold greentext**",
			expected:   `<span class="greentext">&gt;<strong>bold greentext</strong></span>`,
			hasPayload: true,
		},
		{
			name:       "greentext with inline code",
			input:      ">`code`",
			expected:   `<span class="greentext">&gt;<code>code</code></span>`,
			hasPayload: true,
		},
		{
			name:       "normal then greentext",
			input:      "normal text\n>greentext",
			expected:   `normal text<br><span class="greentext">&gt;greentext</span>`,
			hasPayload: true,
		},

		// Message link edge cases
		{
			name:       "message link in text",
			input:      "see >>123#456 here",
			expected:   `see <a href="/test/123#p456" class="message-link message-link-preview" data-board="test" data-message-id="456" data-thread-id="123">&gt;&gt;123#456</a> here`,
			hasPayload: true,
		},
		{
			name:       "invalid message link format",
			input:      ">>abc#def",
			expected:   "&gt;&gt;abc#def",
			hasPayload: true,
		},
		{
			name:       "partial message link",
			input:      ">>123",
			expected:   "&gt;&gt;123",
			hasPayload: true,
		},
		{
			name:       "message link in greentext",
			input:      ">>>123#456",
			expected:   `<span class="greentext">&gt;<a href="/test/123#p456" class="message-link message-link-preview" data-board="test" data-message-id="456" data-thread-id="123">&gt;&gt;123#456</a></span>`,
			hasPayload: true,
		},
		{
			name:       "link not converted in inline code",
			input:      "`>>123#456`",
			expected:   "<code>&gt;&gt;123#456</code>",
			hasPayload: true,
		},
		{
			name:       "link not converted in code block",
			input:      "```\n>>123#456\n```",
			expected:   "<pre><code>&gt;&gt;123#456</code></pre>",
			hasPayload: true,
		},

		// Additional greentext edge cases
		{
			name:       "double greater-than alone",
			input:      ">>",
			expected:   `<span class="greentext">&gt;&gt;</span>`,
			hasPayload: true,
		},
		{
			name:       "triple greater-than alone",
			input:      ">>>",
			expected:   `<span class="greentext">&gt;&gt;&gt;</span>`,
			hasPayload: true,
		},
		// Code block edge cases
		{
			name:       "code block with text after opener",
			input:      "```hello\ncode\n```",
			expected:   "<pre><code>code</code></pre>",
			hasPayload: true,
		},

		// Whitespace in inline formatting
		{
			name:       "inline code with only space",
			input:      "` `",
			expected:   "` `",
			hasPayload: true,
		},

		// Block transitions
		{
			name:       "greentext then code block",
			input:      ">green\n```\ncode\n```",
			expected:   `<span class="greentext">&gt;green</span><br><pre><code>code</code></pre>`,
			hasPayload: true,
		},
		{
			name:       "code block then greentext",
			input:      "```\ncode\n```\n>green",
			expected:   `<pre><code>code</code></pre><br><span class="greentext">&gt;green</span>`,
			hasPayload: true,
		},

		// Adjacent same markers (previously caused panic)
		{
			name:       "adjacent same markers",
			input:      "**a****b**",
			expected:   "<strong>a</strong><strong>b</strong>",
			hasPayload: true,
		},
		{
			name:       "triple asterisk",
			input:      "***text***",
			expected:   "<strong>*text</strong>*", // limitation: longest match first
			hasPayload: true,
		},
		{
			name:       "multiple bold sections",
			input:      "**a**b**c**",
			expected:   "<strong>a</strong>b<strong>c</strong>",
			hasPayload: true,
		},

		// Unicode
		{
			name:       "unicode in text",
			input:      "привет мир",
			expected:   "привет мир",
			hasPayload: true,
		},
		{
			name:       "unicode in bold",
			input:      "**привет**",
			expected:   "<strong>привет</strong>",
			hasPayload: true,
		},
		{
			name:       "unicode in greentext",
			input:      ">привет",
			expected:   `<span class="greentext">&gt;привет</span>`,
			hasPayload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := domain.Message{
				MessageMetadata: domain.MessageMetadata{
					Board:    "test",
					ThreadId: 1,
					Id:       1,
				},
				Text: tt.input,
			}

			result, _, hasPayload, err := tp.ProcessMessage(msg)
			if err != nil {
				t.Fatalf("ProcessMessage returned error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Input:\n%q\n\nExpected:\n%q\n\nGot:\n%q", tt.input, tt.expected, result)
			}

			if hasPayload != tt.hasPayload {
				t.Errorf("hasPayload = %v, want %v", hasPayload, tt.hasPayload)
			}
		})
	}
}

func TestGreentextDoesNotMatchMessageLinks(t *testing.T) {
	cfg := &config.Public{
		MaxRepliesPerMessage: 10,
	}
	tp := New(cfg)

	msg := domain.Message{
		MessageMetadata: domain.MessageMetadata{
			Board:    "test",
			ThreadId: 123,
			Id:       1,
		},
		Text: "Check this >>123#456 post",
	}

	result, replies, _, err := tp.ProcessMessage(msg)
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}

	// Should have one reply link
	if len(replies) != 1 {
		t.Errorf("Expected 1 reply link, got %d", len(replies))
	}

	// Should not have greentext class
	if strings.Contains(result, "greentext") {
		t.Errorf("Message links should not be parsed as greentext. Got: %s", result)
	}

	// Should have message-link class
	if !strings.Contains(result, "message-link") {
		t.Errorf("Expected message-link class in output. Got: %s", result)
	}
}

func TestReplyDeduplication(t *testing.T) {
	cfg := &config.Public{
		MaxRepliesPerMessage: 10,
	}
	tp := New(cfg)

	msg := domain.Message{
		MessageMetadata: domain.MessageMetadata{
			Board:    "test",
			ThreadId: 1,
			Id:       1,
		},
		Text: ">>123#456 and >>123#456 again",
	}

	_, replies, _, err := tp.ProcessMessage(msg)
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}

	// Should only have one reply (deduplicated)
	if len(replies) != 1 {
		t.Errorf("Expected 1 reply (deduplicated), got %d", len(replies))
	}
}

func TestReplyLimit(t *testing.T) {
	cfg := &config.Public{
		MaxRepliesPerMessage: 2,
	}
	tp := New(cfg)

	msg := domain.Message{
		MessageMetadata: domain.MessageMetadata{
			Board:    "test",
			ThreadId: 1,
			Id:       1,
		},
		Text: ">>1#1 >>2#2 >>3#3 >>4#4",
	}

	_, replies, _, err := tp.ProcessMessage(msg)
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}

	// Should only have 2 replies (limited)
	if len(replies) != 2 {
		t.Errorf("Expected 2 replies (limited), got %d", len(replies))
	}
}
