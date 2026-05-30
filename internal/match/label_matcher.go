package match

import "k8s.io/apimachinery/pkg/labels"

// LabelMatcher is a read-only version of [labels.Selector].
// It provides only the ability to match labels.
type LabelMatcher interface {
	// Matches is the same as [labels.Selector.Matches].
	Matches(labels.Labels) bool
}

// Nothing returns true for matchers which will never match any objects.
// It otherwise returns false.
func Nothing(matcher LabelMatcher) bool {
	return matcher == labels.Nothing()
}
