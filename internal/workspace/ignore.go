package workspace

import (
	"path"
	"strings"
)

// ignoreMatcher implements a small, dependency-free matcher for the patterns
// listed in DefaultIgnore plus user-supplied additions. Patterns are matched
// against the slash-separated relative path of every entry. A pattern that
// does not contain a slash matches by basename anywhere in the tree; otherwise
// it matches by exact relative path.
type ignoreMatcher struct {
	basenames map[string]struct{}
	exact     map[string]struct{}
}

func newIgnoreMatcher(extra []string) *ignoreMatcher {
	matcher := &ignoreMatcher{
		basenames: map[string]struct{}{},
		exact:     map[string]struct{}{},
	}
	for _, pattern := range append(append([]string{}, DefaultIgnore...), extra...) {
		matcher.add(pattern)
	}
	return matcher
}

func (m *ignoreMatcher) add(pattern string) {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.TrimPrefix(pattern, "./")
	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	if pattern == "" {
		return
	}
	if strings.Contains(pattern, "/") {
		m.exact[path.Clean(pattern)] = struct{}{}
		return
	}
	m.basenames[pattern] = struct{}{}
}

func (m *ignoreMatcher) match(rel string, isDir bool) bool {
	if rel == "" {
		return false
	}
	clean := path.Clean(rel)
	if _, ok := m.exact[clean]; ok {
		return true
	}
	base := path.Base(clean)
	if _, ok := m.basenames[base]; ok {
		return true
	}
	return false
}
