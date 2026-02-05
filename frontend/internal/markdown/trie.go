package markdown

// Rule is a minimal struct for trie matching (just needs marker)
type Rule interface {
	Marker() string
}

// TrieNode represents a node in the prefix tree
type TrieNode struct {
	children map[byte]*TrieNode
	rule     Rule // non-nil if valid marker ends here
}

type Match struct {
	Rule Rule
	Len  int
}

// Trie is a prefix tree for fast marker matching
type Trie struct {
	root *TrieNode
}

func newTrie() *Trie {
	return &Trie{root: &TrieNode{children: make(map[byte]*TrieNode)}}
}

func (t *Trie) insert(rule Rule) {
	node := t.root
	for i := 0; i < len(rule.Marker()); i++ {
		c := rule.Marker()[i]
		if node.children[c] == nil {
			node.children[c] = &TrieNode{children: make(map[byte]*TrieNode)}
		}
		node = node.children[c]
	}
	node.rule = rule
}

// Match returns the longest matching rule and its length, or nil if no match
func (t *Trie) Match(text string, pos int) []Match {
	node := t.root
	var matches []Match

	for i := pos; i < len(text); i++ {
		c := text[i]
		next := node.children[c]
		if next == nil {
			break
		}
		node = next
		if node.rule != nil {
			matches = append(matches, Match{Rule: node.rule, Len: i - pos + 1})
		}
	}
	return matches
}
