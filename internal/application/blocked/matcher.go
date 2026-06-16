package blocked

import (
	"strings"
	"unicode"

	"github.com/samber/lo"
)

type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	output   []uint
}

type ACmatcher struct {
	root *acNode
}

func NewACmatcher(words map[uint]string) *ACmatcher {
	m := &ACmatcher{root: &acNode{children: make(map[rune]*acNode)}}
	for id, word := range words {
		word = strings.ToLower(word)
		node := m.root
		for _, r := range word {
			child, ok := node.children[r]
			if !ok {
				child = &acNode{children: make(map[rune]*acNode)}
				node.children[r] = child
			}
			node = child
		}
		node.output = append(node.output, id)
	}
	queue := lo.Values(m.root.children)
	for _, child := range queue {
		child.fail = m.root
	}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for r, child := range parent.children {
			fail := parent.fail
			for fail != nil {
				if next, ok := fail.children[r]; ok {
					child.fail = next
					break
				}
				fail = fail.fail
			}
			if child.fail == nil {
				child.fail = m.root
			}
			child.output = append(child.output, child.fail.output...)
			queue = append(queue, child)
		}
	}
	return m
}

func (m *ACmatcher) Match(text string) []uint {
	matched := make(map[uint]struct{})
	node := m.root
	for _, r := range text {
		r = unicode.ToLower(r)
		for node != m.root && node.children[r] == nil {
			node = node.fail
		}
		if next, ok := node.children[r]; ok {
			node = next
		}
		for _, id := range node.output {
			matched[id] = struct{}{}
		}
	}
	return lo.Keys(matched)
}
