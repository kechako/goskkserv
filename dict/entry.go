package dict

import (
	"fmt"
	"strings"
)

type Candidate interface {
	Text() string
	Annotation() string
	fmt.Stringer
}

type candidate struct {
	text       string
	annotation string
}

var _ Candidate = (*candidate)(nil)

func (c *candidate) Text() string {
	return c.text
}

func (c *candidate) Annotation() string {
	return c.annotation
}

func (c *candidate) String() string {
	if len(c.annotation) == 0 {
		return c.text
	}

	var s strings.Builder
	s.Grow(len(c.text) + len(c.annotation) + 2)

	s.WriteString(c.text)
	s.WriteString("; ")
	s.WriteString(c.annotation)

	return s.String()
}

type entry struct {
	candidates []*candidate
	candSet    map[string]struct{}
}

func newEntry() *entry {
	return &entry{
		candSet: make(map[string]struct{}),
	}
}

func (e *entry) add(text, annotation string) bool {
	if _, ok := e.candSet[text]; ok {
		return false
	}

	cand := &candidate{
		text:       text,
		annotation: annotation,
	}
	e.candSet[text] = struct{}{}
	e.candidates = append(e.candidates, cand)

	return true
}

func (e *entry) Candidates() []Candidate {
	if len(e.candidates) == 0 {
		return nil
	}

	candidates := make([]Candidate, len(e.candidates))
	for i, c := range e.candidates {
		candidates[i] = c
	}

	return candidates
}
