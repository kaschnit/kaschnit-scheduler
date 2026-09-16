package queue

import (
	"errors"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/match"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
)

// PreemptionEvaluator evaluates preemption for pods belonging to queues.
type PreemptionEvaluator struct {
	queueMgr      *Manager
	preemptor     *corev1.Pod
	preemptorQ    *Queue
	preemptorPrio int32
}

// NewPreemptionEvaluator creates a new [PreemptionEvaluator].
func NewPreemptionEvaluator(queueMgr *Manager, preemptor *corev1.Pod) PreemptionEvaluator {
	return PreemptionEvaluator{
		queueMgr:      queueMgr,
		preemptor:     preemptor,
		preemptorQ:    queueMgr.Get(preemptor),
		preemptorPrio: corev1helpers.PodPriority(preemptor),
	}
}

// IsConfiguredForPreemption returns true if the queue is configured for preemption.
func (pe PreemptionEvaluator) IsConfiguredForPreemption() bool {
	return pe.preemptorQ != nil
}

// CanPodPreemptOthers returns whether the pod is allowed to preempt other pods.
// This is based only on the configuration of the pod and its queue, not other queues or pods.
func (pe PreemptionEvaluator) CanPodPreemptOthers() bool {
	if pe.preemptor == nil {
		return false
	}

	q := pe.queueMgr.Get(pe.preemptor)
	if q == nil {
		return false
	}

	preemptionCfg := q.PreemptionConfig()

	// Can q prempt at all?
	if !preemptionCfg.Preempts.CanPreempt() {
		return false
	}

	// Does q allow pod to preempt others at all?
	if !preemptionCfg.Preempts.FromPods.Matches(labels.Set(pe.preemptor.Labels)) {
		return false
	}

	return true
}

// IsPreemptionAllowed returns whether preemptor can preempt victim.
func (pe PreemptionEvaluator) IsPreemptionAllowed(victim *corev1.Pod) bool {
	// If any of the inputs are nil, preemption never makes sense.
	if pe.preemptor == nil || pe.preemptorQ == nil || victim == nil {
		return false
	}

	// Not a victim if it's same or higher priority than the preemptor.
	// This is a hard requirement to prevent endless preemption loop.
	if corev1helpers.PodPriority(victim) >= pe.preemptorPrio {
		return false
	}

	// Check if preemptor matches victim (egress / preempts rules).
	preemptorCfg := pe.preemptorQ.PreemptionConfig()
	// Can preemptorQ preempt at all?
	if !preemptorCfg.Preempts.CanPreempt() {
		return false
	}
	// Does preemptorQ allow preemptor pod to preempt others at all?
	if !preemptorCfg.Preempts.FromPods.Matches(labels.Set(pe.preemptor.Labels)) {
		return false
	}
	// Can preemptorQ's pods preempt victim pod specificially?
	if !preemptorCfg.Preempts.ToPods.Matches(labels.Set(victim.Labels)) {
		return false
	}

	// Check if victim matches preemptor (ingress / preemptedBy rules).
	victimQ := pe.queueMgr.Get(victim)
	if victimQ == nil {
		return false
	}
	toPreemptionCfg := victimQ.PreemptionConfig()
	// Can victimQ be preempted at all?
	if !toPreemptionCfg.PreemptedBy.CanBePreempted() {
		return false
	}
	// Does victimQ allow victim pod be preempted at all?
	if !toPreemptionCfg.PreemptedBy.ToPods.Matches(labels.Set(victim.Labels)) {
		return false
	}
	// Can victimQ's pods be preempted by preemptor pod specifically?
	if !toPreemptionCfg.PreemptedBy.FromPods.Matches(labels.Set(pe.preemptor.Labels)) {
		return false
	}

	return true
}

// NewPreemptionConfigFromSpec converts the API preemption spec to [PreemptionConfig].
func NewPreemptionConfigFromSpec(spec schedv1.PreemptionSpec) (PreemptionConfig, error) {
	var errs error

	preempts, err := makePreemptionRuleFromSpecRule(spec.Preempts)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	preemptedBy, err := makePreemptionRuleFromSpecRule(spec.PreemptedBy)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	return PreemptionConfig{
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
