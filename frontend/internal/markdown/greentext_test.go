package markdown

import (
	"strings"
	"testing"

	"github.com/itchan-dev/itchan/shared/config"
	"github.com/itchan-dev/itchan/shared/domain"
)

func TestGreentext(t *testing.T) {
	cfg := &config.Public{}
	tp := New(cfg)

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "single line greentext",
			input:    ">implying",
			contains: `<span class="greentext">&gt;implying</span>`,
		},
		{
			name:     "multi line greentext",
			input:    ">be me\n>using imageboard",
			contains: `<span class="greentext">&gt;be me<br>&gt;using imageboard</span>`,
		},
		{
			name:     "greentext with blank line separator",
			input:    ">first line\n\n>second line",
			contains: `<span class="greentext">&gt;first line</span>`,
		},
		{
			name:     "not greentext - message link",
			input:    ">>123#456",
			contains: `&gt;&gt;123#456`,
		},
		{
			name:     "greentext and normal text",
			input:    ">greentext line\nnormal text",
			contains: `<span class="greentext">&gt;greentext line</span>`,
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

			result, _, _ := tp.ProcessMessage(msg)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected output to contain:\n%s\n\nGot:\n%s", tt.contains, result)
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

	result, replies, _ := tp.ProcessMessage(msg)

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
