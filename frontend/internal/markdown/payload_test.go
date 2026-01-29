package markdown

import (
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

func TestHasPayload(t *testing.T) {
	cfg := &config.Public{}
	tp := New(cfg)

	tests := []struct {
		name       string
		input      string
		hasPayload bool
	}{
		{
			name:       "normal text",
			input:      "hello world",
			hasPayload: true,
		},
		{
			name:       "empty string",
			input:      "",
			hasPayload: false,
		},
		{
			name:       "only whitespace",
			input:      "   \n\n   ",
			hasPayload: false,
		},
		{
			name:       "empty code block",
			input:      "```\n```",
			hasPayload: false,
		},
		{
			name:       "empty code block with whitespace",
			input:      "```\n   \n```",
			hasPayload: false,
		},
		{
			name:       "code block with content",
			input:      "```\ncode here\n```",
			hasPayload: true,
		},
		{
			name:       "only bold formatting",
			input:      "**  **",
			hasPayload: true, // Not parsed as bold, stays as literal text
		},
		{
			name:       "bold with content",
			input:      "**hello**",
			hasPayload: true,
		},
		{
			name:       "only strikethrough",
			input:      "~~  ~~",
			hasPayload: true, // Not parsed as strikethrough, stays as literal text
		},
		{
			name:       "greentext",
			input:      ">greentext",
			hasPayload: true,
		},
		{
			name:       "empty greentext",
			input:      ">",
			hasPayload: true, // The ">" itself is content
		},
		{
			name:       "inline code",
			input:      "`code`",
			hasPayload: true,
		},
		{
			name:       "empty inline code",
			input:      "``",
			hasPayload: true, // Not parsed as code, stays as literal text
		},
		{
			name:       "message link",
			input:      ">>123#456",
			hasPayload: true, // Link text is content
		},
		{
			name:       "multiple empty blocks",
			input:      "```\n```\n\n```\n```",
			hasPayload: false,
		},
		{
			name:       "mixed empty and content",
			input:      "```\n```\n\nhello",
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

			processedText, _, hasPayload := tp.ProcessMessage(msg)

			if hasPayload != tt.hasPayload {
				t.Errorf("hasPayload = %v, want %v\nProcessed HTML: %q", hasPayload, tt.hasPayload, processedText)
			}
		})
	}
}
