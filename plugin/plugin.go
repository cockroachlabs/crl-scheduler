package plugin

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

const (
	Name             = "ZonalDistribution"
	ZoneLabel        = "failure-domain.beta.kubernetes.io/zone"
	StatefulSetLabel = "statefulset.kubernetes.io/pod-name"
	VZDKey           = "VZD"
)

type ZonalDistributionPlugin struct {
	handle    framework.FrameworkHandle
	clientset kubernetes.Interface
}

var _ framework.PreFilterPlugin = &ZonalDistributionPlugin{}
var _ framework.FilterPlugin = &ZonalDistributionPlugin{}

func (p *ZonalDistributionPlugin) Name() string {
	return Name
}

func (p *ZonalDistributionPlugin) PreFilter(pc *framework.PluginContext, pod *corev1.Pod) *framework.Status {
	var ownerReference *metav1.OwnerReference

	for i, or := range pod.GetOwnerReferences() {
		if or.Kind == "StatefulSet" {
			ownerReference = &pod.GetOwnerReferences()[i]
			break
		}
	}

	if ownerReference == nil {
		return framework.NewStatus(framework.Error, "could not find owning sts")
	}

	sts, err := p.clientset.AppsV1().StatefulSets(pod.Namespace).Get(ownerReference.Name, metav1.GetOptions{})
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Errorf("could not find owning sts: %w", err).Error())
	}

	// VZD is built here to avoid hitting the k8s API quite so hard.
	vzd, err := BuildVolumeZonalDistribution(p.clientset, sts)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	pc.Lock()
	defer pc.Unlock()

	pc.Write(VZDKey, vzd)

	klog.Infof("Built Zonal Volume Distribution: %#v", vzd)

	return framework.NewStatus(framework.Success, "")
}

func (p *ZonalDistributionPlugin) Filter(pc *framework.PluginContext, pod *corev1.Pod, nodeName string) *framework.Status {
	ordinal := PodOrdinal(pod.Name)

	// Ignore non-stateful set pods
	if _, ok := pod.Labels[StatefulSetLabel]; !ok || ordinal < 0 {
		return framework.NewStatus(framework.Success, "")
	}

	nodeZone, ok := p.handle.NodeInfoSnapshot().NodeInfoMap[nodeName].Node().Labels[ZoneLabel]

	// Ignore nodes without a zone
	if !ok {
		return framework.NewStatus(framework.Unschedulable, "no zonal information found")
	}

	pc.RLock()
	defer pc.RUnlock()

	// Extract our precomputed values from the context
	vzdData, err := pc.Read(VZDKey)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	// scheduler runtime will catch the panic if something has gone
	// horribly wrong
	vzd := (vzdData).(map[Zone]map[uint]bool)

	// Would be nice to compute this in PreFilter but
	// NodeInfoMap isn't populated in PreFilter.
	nodes := Nodes(p.handle.NodeInfoSnapshot().NodeInfoMap)

	slices := BuildZonalTopology(nodes, vzd)

	idealZone := slices.IdealZone(uint(ordinal))

	// log out all information required to recreate various cases
	klog.Infof("Built Node List: %#v", nodes)
	klog.Infof("Built Ideal Zonal Topology: %#v", slices)
	klog.Infof("Ideal zone for ordinal %d is %s", ordinal, idealZone)

	// If this node is in the ideal zone for this ordinal, allow scheduling
	if idealZone == nodeZone {
		return framework.NewStatus(framework.Success, "")
	}

	// If this ordinal is being scheduled into a zone that contains it's PVCs
	// allow it to be scheduled.
	// NOTE: In the future it may be desirable to prevent pods from being scheduled if their volumes'
	// zones don't match up with their ideal zone. This functionality could be added behind a CLI flag
	// IE --inforce or --suggest
	// For example: When correcting a cluster's topology, being able to force "bad" pods into a stuck/waiting state
	// allows the "bad" PVC/PV to be deleted without race conditions.
	if vzd[nodeZone][uint(ordinal)] {
		klog.Warningf("pod %s is allowed onto node %s in %s to follow it's volumes", pod.Name, nodeName, nodeZone)
		return framework.NewStatus(framework.Success, "")
	}

	return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("not in ideal zone %s", idealZone))
}

func New(configuration *runtime.Unknown, f framework.FrameworkHandle) (framework.Plugin, error) {
	// This is effectively a hack. Newer versions (1.17+) of the scheduler runtime provide access
	// to watchers via the FrameworkHandle. For now, inject our own k8s clientset set to gain access
	// to volume information
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("could not build in cluster rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not build clientset with in cluster rest config: %w", err)
	}

	return &ZonalDistributionPlugin{handle: f, clientset: clientset}, nil
}
