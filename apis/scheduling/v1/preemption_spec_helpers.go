package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PreemptionEverything returns a rule that allows preemption of everything that is
// allowed to be preempted.
func PreemptionEverything() PreemptionRule {
	return PreemptionRule{
		FromPods: &metav1.LabelSelector{},
		ToPods:   &metav1.LabelSelector{},
	}
}

// PreemptionNothing returns a rules that prevents preempting.
func PreemptionNothing() PreemptionRule {
	return PreemptionRule{}
}
