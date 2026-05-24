package main

import (
	"context"
	"os"

	"github.com/kaschnit/kaschnit-scheduler/internal/plugin/quotaawarepreempt"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/cli"
	"k8s.io/klog/v2"
	fwk "k8s.io/kube-scheduler/framework"
	scheduler "k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	schedruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

func main() {
	command := scheduler.NewSchedulerCommand(
		scheduler.WithPlugin(
			quotaawarepreempt.PluginName,
			func(ctx context.Context, configuration runtime.Object, fh fwk.Handle) (fwk.Plugin, error) {
				logger := klog.FromContext(ctx).WithValues("plugin", quotaawarepreempt.PluginName)

				fts := feature.NewSchedulerFeaturesFromGates(utilfeature.DefaultFeatureGate)

				logger.Info("Starting plugin",
					"features", fts)

				factory := schedruntime.FactoryAdapter(fts, quotaawarepreempt.NewPlugin)
				return factory(ctx, configuration, fh)
			},
		),
	)

	exitStatus := cli.Run(command)
	os.Exit(exitStatus)
}
