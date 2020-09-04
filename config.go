package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"gopkg.in/yaml.v2"
	kubeschedulerconfigv1alpha1 "k8s.io/kube-scheduler/config/v1alpha1"
	"k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	kubeschedulerscheme "k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
)

const (
	name       = "default"
	tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

func WriteInClusterKubeConfig(path string) error {
	ca, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return fmt.Errorf("could not read root CA: %w", err)
	}

	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return fmt.Errorf("could not read k8s token: %w", err)
	}

	kubeAPIServer := "https://" + net.JoinHostPort(
		os.Getenv("KUBERNETES_SERVICE_HOST"),
		os.Getenv("KUBERNETES_SERVICE_PORT"),
	)

	config := map[string]interface{}{
		"apiVersion":      "v1",
		"kind":            "Config",
		"current-context": name,
		"clusters": []interface{}{
			map[string]interface{}{
				"name": name,
				"cluster": map[string]interface{}{
					"certificate-authority-data": base64.StdEncoding.EncodeToString(ca),
					"server":                     kubeAPIServer,
				},
			},
		},
		"contexts": []interface{}{
			map[string]interface{}{
				"name": name,
				"context": map[string]interface{}{
					"cluster": name,
					"user":    name,
				},
			},
		},
		"users": []interface{}{
			map[string]interface{}{
				"name": name,
				"user": map[string]interface{}{
					"token": string(token),
				},
			},
		},
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	if err := ioutil.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("could not write kubeconfig to %s: %w", path, err)
	}

	return nil
}

func WriteSchedulerConfig(name, path, kubeConfigPath string) error {
	// Create a default config with all fields so we can override them
	cfgv1alpha1 := kubeschedulerconfigv1alpha1.KubeSchedulerConfiguration{}
	kubeschedulerscheme.Scheme.Default(&cfgv1alpha1)
	cfg := kubeschedulerconfig.KubeSchedulerConfiguration{}
	if err := kubeschedulerscheme.Scheme.Convert(&cfgv1alpha1, &cfg, nil); err != nil {
		return err
	}

	// Custom name ensures we only target pods with this name
	cfg.SchedulerName = name

	// The kubeconfig previously written
	cfg.ClientConnection.Kubeconfig = kubeConfigPath

	// Only running a single replica
	cfg.LeaderElection.LeaderElect = false

	// Enabled our custom plugin
	cfg.Plugins = &kubeschedulerconfig.Plugins{
		PreFilter: &kubeschedulerconfig.PluginSet{
			Enabled: []kubeschedulerconfig.Plugin{
				{Name: "ZonalDistribution"},
			},
		},
		Filter: &kubeschedulerconfig.PluginSet{
			Enabled: []kubeschedulerconfig.Plugin{
				{Name: "ZonalDistribution"},
			},
		},
	}

	// Use the same code path as the scheduler's --write-config-to flag
	return options.WriteConfigFile(path, &cfg)
}
