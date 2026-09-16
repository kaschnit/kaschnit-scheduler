package queue

import (
	"errors"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/match"
)

func NewPreemptionConfigFromSpec(spec schedv1.PreemptionSpec) (*PreemptionConfig, error) {
	var errs error

	preempts, err := makePreemptionRuleFromSpecRule(spec.Preempts)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	preemptedBy, err := makePreemptionRuleFromSpecRule(spec.PreemptedBy)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return &PreemptionConfig{
		Preempts:    PreemptsRule(preempts),
		PreemptedBy: PreemptedByRule(preemptedBy),
	}, errs
}

type PreemptionConfig struct {
	Preempts    PreemptsRule
	PreemptedBy PreemptedByRule
}

type PreemptsRule preemptionRule

func (rule PreemptsRule) CanPreempt() bool {
	return rule.FromPods != nil && rule.ToPods != nil
}

type PreemptedByRule preemptionRule

func (rule PreemptedByRule) CanBePreempted() bool {
	return rule.FromPods != nil && rule.ToPods != nil
}

func makePreemptionRuleFromSpecRule(rule schedv1.PreemptionRule) (preemptionRule, error) {
	var errs error

	fromPods, err := match.LabelSelectorAsMatcherOrNothing(rule.FromPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	toPods, err := match.LabelSelectorAsMatcherOrNothing(rule.ToPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return preemptionRule{
		FromPods: fromPods,
		ToPods:   toPods,
	}, errs
}

type preemptionRule struct {
	FromPods match.LabelMatcher
	ToPods   match.LabelMatcher
}
