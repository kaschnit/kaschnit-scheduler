package queue

import (
	"errors"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/match"
)

type PreemptionConfig struct {
	preempts    preemptsRule
	preemptedBy preemptedByRule
}

func NewPreemptionConfigFromSpec(spec schedv1.PreemptionSpec) (*PreemptionConfig, error) {
	var errs error

	preempts, err := makePreemptsRuleFromSpec(spec.Preempts)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	preemptedBy, err := makePreemptedByRuleFromSpec(spec.PreemptedBy)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return &PreemptionConfig{
		preempts:    preempts,
		preemptedBy: preemptedBy,
	}, errs
}

type preemptsRule struct {
	fromPods match.LabelMatcher
	toPods   match.LabelMatcher
}

func makePreemptsRuleFromSpec(rule schedv1.PreemptsRule) (preemptsRule, error) {
	var errs error

	fromPods, err := match.LabelSelectorAsMatcherOrNothing(rule.FromPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	toPods, err := match.LabelSelectorAsMatcherOrNothing(rule.ToPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return preemptsRule{
		fromPods: fromPods,
		toPods:   toPods,
	}, errs
}

func (rule preemptsRule) canPreempt() bool {
	return rule.fromPods != nil && rule.toPods != nil
}

type preemptedByRule struct {
	fromPods match.LabelMatcher
	toPods   match.LabelMatcher
}

func makePreemptedByRuleFromSpec(rule schedv1.PreemptedByRule) (preemptedByRule, error) {
	var errs error

	fromPods, err := match.LabelSelectorAsMatcherOrNothing(rule.FromPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	toPods, err := match.LabelSelectorAsMatcherOrNothing(rule.ToPods)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return preemptedByRule{
		fromPods: fromPods,
		toPods:   toPods,
	}, errs
}

func (rule preemptedByRule) canBePreempted() bool {
	return rule.fromPods != nil && rule.toPods != nil
}
