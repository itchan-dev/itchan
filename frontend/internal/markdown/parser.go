package markdown

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type lenientParagraphParser struct {
}

// check interface implementation
var _ parser.BlockParser = &lenientParagraphParser{}

func NewLenientParagraphParser() parser.BlockParser {
	return &lenientParagraphParser{}
}

func (b *lenientParagraphParser) Trigger() []byte {
	return nil
}

// Consume first line of the paragraph
func (b *lenientParagraphParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	// Get the segment for the current line. A segment is a slice of the source text.
	_, segment := reader.PeekLine()
	node := ast.NewParagraph()
	// Append line to segment and advance pointer past the line we just consumed.
	node.Lines().Append(segment)
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

// Continue consumes all SUBSEQUENT lines for the paragraph.
func (b *lenientParagraphParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	// Peek at the next line to see what it is.
	line, segment := reader.PeekLine()

	// If the line is blank, this paragraph is finished.
	if util.IsBlank(line) {
		return parser.Close
	}

	// If the line is not blank, append its content to our paragraph node.
	node.Lines().Append(segment)

	// Advance the reader past this line.
	reader.AdvanceToEOL()

	// Continue processing, looking for the next line.
	return (parser.Continue | parser.NoChildren)
}

func (b *lenientParagraphParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	lines := node.Lines()
	if lines.Len() != 0 {
		// trim leading spaces
		for i := 0; i < lines.Len(); i++ {
			l := lines.At(i)
			lines.Set(i, l.TrimLeftSpace(reader.Source()))
		}

		// trim trailing spaces
		length := lines.Len()
		lastLine := node.Lines().At(length - 1)
		node.Lines().Set(length-1, lastLine.TrimRightSpace(reader.Source()))
	}
	if lines.Len() == 0 {
		node.Parent().RemoveChild(node.Parent(), node)
		return
	}
}

func (b *lenientParagraphParser) CanInterruptParagraph() bool {
	return false
}

func (b *lenientParagraphParser) CanAcceptIndentedLine() bool {
	return true
}
