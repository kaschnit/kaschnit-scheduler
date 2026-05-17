package main

import (
	"os"

	"github.com/kaschnit/custom-scheduler/internal/plugin/quotaawarepreempt"
	"k8s.io/component-base/cli"
	scheduler "k8s.io/kubernetes/cmd/kube-scheduler/app"
)

// +kubebuilder:rbac:groups="",resources=namespaces;configmaps;replicationcontrollers;services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,resourceNames=custom-scheduler,verbs=get;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=create
// +kubebuilder:rbac:groups="",resources=endpoints,resourceNames=custom-scheduler,verbs=get;update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=delete;get;list;watch;update
// +kubebuilder:rbac:groups="",resources=bindings;pods/binding,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=patch;update
// +kubebuilder:rbac:groups=apps;extensions,resources=replicasets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;persistentvolumes,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=resource.k8s.io,resources=deviceclasses;resourceclaims;resourceslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csinodes;storageclasses;csidrivers;csistoragecapacities;volumeattachments,verbs=get;list;watch
// +kubebuilder:rbac:groups=topology.node.k8s.io,resources=noderesourcetopologies,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=podgroups;elasticquotas;podgroups/status;elasticquotas/status,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=scheduling.k8s.io,resources=priorityclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduling.kaschnit.github.io,resources=queues,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduling.kaschnit.github.io,resources=queues/status,verbs=get;update;patch

func main() {
	command := scheduler.NewSchedulerCommand(
		scheduler.WithPlugin(quotaawarepreempt.PluginName, quotaawarepreempt.NewPlugin),
	)

	exitStatus := cli.Run(command)
	os.Exit(exitStatus)
}
