package labelutil

import "k8s.io/apimachinery/pkg/labels"

// Matcher is a read-only version of [labels.Selector].
// It provides only the ability to match labels.
type Matcher interface {
	// Matches is the same as [labels.Selector.Matches].
	Matches(labels.Labels) bool
}

// MatchesNothing returns true for matchers which will never match any objects.
// It otherwise returns false.
func MatchesNothing(matcher Matcher) bool {
	return matcher == labels.Nothing()
}
