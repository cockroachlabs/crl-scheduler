package main

import (
	"fmt"
	"os"

	"github.com/cockroachlabs/crl-scheduler/plugin"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

const (
	schedulerName  = "crl-scheduler"
	configPath     = "/scheduler-config"
	kubeConfigPath = "/kubeconfig"
)

func main() {
	if err := WriteInClusterKubeConfig(kubeConfigPath); err != nil {
		klog.Fatal(err)
	}

	if err := WriteSchedulerConfig(schedulerName, configPath, kubeConfigPath); err != nil {
		klog.Fatal(err)
	}

	// Force the usage of our custom config
	os.Args = append(os.Args, fmt.Sprintf("--config=%s", configPath))

	command := app.NewSchedulerCommand(
		// Register our custom plugin
		app.WithPlugin(plugin.Name, plugin.New),
	)

	if err := command.Execute(); err != nil {
		klog.Fatal(err)
	}
}
