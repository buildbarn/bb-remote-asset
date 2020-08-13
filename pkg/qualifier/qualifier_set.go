package qualifier

import (
	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
)

var exists = struct{}{}

// Set implements a HashSet of qualifier names
type Set map[string]struct{}

// NewSet creates a new Set from a list of qualifier names
func NewSet(names []string) Set {
	s := Set{}
	for _, n := range names {
		s[n] = exists
	}
	return s
}

// IsEmpty checks if the Set is empty
func (s Set) IsEmpty() bool {
	return len(s) == 0
}

// Contains checks if the set contains a given Qualifier Name
func (s Set) Contains(q string) bool {
	_, ok := s[q]
	return ok
}

// Add adds a qualifier name to the set
func (s Set) Add(q string) {
	s[q] = exists
}

// Difference calculates the Set difference a \ b.
func Difference(a Set, b Set) Set {
	diff := Set{}
	for k := range a {
		if !b.Contains(k) {
			diff.Add(k)
		}
	}
	return diff
}

// QualifiersToSet converts an array of qualifiers into a Set of names
func QualifiersToSet(qualifiers []*remoteasset.Qualifier) Set {
	s := Set{}
	for _, q := range qualifiers {
		s.Add(q.Name)
	}
	return s
}
