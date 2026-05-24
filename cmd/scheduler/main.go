package main

import (
	"context"
	"os"

	"github.com/kaschnit/kaschnit-scheduler/internal/plugin/quotaawarepreempt"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/cli"
	fwk "k8s.io/kube-scheduler/framework"
	scheduler "k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

func main() {
	command := scheduler.NewSchedulerCommand(
		scheduler.WithPlugin(
			quotaawarepreempt.PluginName,
			func(ctx context.Context, obj runtime.Object, fh fwk.Handle) (fwk.Plugin, error) {
				fts := feature.NewSchedulerFeaturesFromGates(utilfeature.DefaultFeatureGate)
				factory := frameworkruntime.FactoryAdapter(fts, quotaawarepreempt.NewPlugin)
				return factory(ctx, obj, fh)
			},
		),
	)

	exitStatus := cli.Run(command)
	os.Exit(exitStatus)
}
