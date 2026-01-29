package markdown

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Greentext represents a greentext block (lines starting with >)
type Greentext struct {
	ast.BaseBlock
}

func (n *Greentext) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

var KindGreentext = ast.NewNodeKind("Greentext")

func (n *Greentext) Kind() ast.NodeKind {
	return KindGreentext
}

func NewGreentext() *Greentext {
	return &Greentext{}
}

// greentextParser parses lines starting with > as greentext
type greentextParser struct{}

func NewGreentextParser() parser.BlockParser {
	return &greentextParser{}
}

func (b *greentextParser) Trigger() []byte {
	return []byte{'>'}
}

func (b *greentextParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()

	// Must start with >
	if len(line) == 0 || line[0] != '>' {
		return nil, parser.NoChildren
	}

	// Skip >> patterns (those are message links like >>123#456)
	if len(line) > 1 && line[1] == '>' {
		return nil, parser.NoChildren
	}

	node := NewGreentext()
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return node, parser.NoChildren
}

func (b *greentextParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, segment := reader.PeekLine()

	// Blank line ends greentext
	if util.IsBlank(line) {
		return parser.Close
	}

	// Next line must also start with > to continue (but not >>)
	if len(line) > 0 && line[0] == '>' && !(len(line) > 1 && line[1] == '>') {
		node.Lines().Append(segment)
		reader.Advance(segment.Len() - 1)
		return parser.Continue | parser.NoChildren
	}

	return parser.Close
}

func (b *greentextParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	// Nothing special needed on close
}

func (b *greentextParser) CanInterruptParagraph() bool {
	return true
}

func (b *greentextParser) CanAcceptIndentedLine() bool {
	return false
}

// GreentextHTMLRenderer renders greentext nodes to HTML
type GreentextHTMLRenderer struct {
	html.Config
}

func NewGreentextHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &GreentextHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

func (r *GreentextHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindGreentext, r.renderGreentext)
}

func (r *GreentextHTMLRenderer) renderGreentext(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<span class="greentext">`)
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			// Get line value and trim trailing newline (handles both \n and \r\n)
			value := bytes.TrimRight(line.Value(source), "\r\n")
			_, _ = w.Write(value)
			if i < lines.Len()-1 {
				_, _ = w.WriteString("<br>")
			}
		}
		_, _ = w.WriteString("</span>")
	}
	return ast.WalkSkipChildren, nil
}
