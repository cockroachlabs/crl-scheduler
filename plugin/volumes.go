package plugin

import (
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const PageSize = 100

// ForAllPersistentVolumes runs cb on the paginated list of PVs, returning early in the case of an error.
func ForAllPeristentVolumes(
	iface typedCoreV1.PersistentVolumeInterface,
	cb func(pv *corev1.PersistentVolume) error,
) error {
	pvs, err := iface.List(metav1.ListOptions{Limit: PageSize})
	if err != nil {
		return err
	}

	for {
		for _, pv := range pvs.Items {
			if err := cb(&pv); err != nil {
				return err
			}
		}

		if pvs.Continue == "" || len(pvs.Items) < PageSize {
			return nil
		}

		pvs, err = iface.List(metav1.ListOptions{Continue: pvs.Continue})
		if err != nil {
			return err
		}
	}
}

// BuildVolumeZonalDistribution finds all PersistentVolumes created by the given StatefulSet and the
// AvaililityZones they were created in. The resulting data structure can be used to tell if a sts ordinal
// has volumes in a specific zone.
// The following would indicate that pod ordinals 0 and 1 both have a PV in zoneA.
// The pods for ordinals 0 and 1 may or may not exist.
// map{
//	  "zoneA": map{0: true, 1: true}
// }
func BuildVolumeZonalDistribution(
	clientset kubernetes.Interface,
	sts *appsv1.StatefulSet,
) (map[Zone]map[uint]bool, error) {
	ret := map[Zone]map[uint]bool{}

	if err := ForAllPeristentVolumes(
		clientset.CoreV1().PersistentVolumes(),
		func(pv *corev1.PersistentVolume) error {
			// Skip PVs that don't have a claim ref
			if pv.Spec.ClaimRef == nil {
				return nil
			}

			// TODO(chrisseto): Could some of this complexity be replaced with
			// field selectors? IE server side filtering for Volume Status

			// If this PV isn't currently bound or pending, ignore it. It may have been released,
			// (the PVC was deleted) which means it is no longer a candidate for scheduling consideration
			if pv.Status.Phase != corev1.VolumePending && pv.Status.Phase != corev1.VolumeBound {
				return nil
			}

			for _, tpl := range sts.Spec.VolumeClaimTemplates {
				// PVCs are named <volumeName>-<statefulsetname>-<ordinal>
				prefix := fmt.Sprintf("%s-%s-", tpl.Name, sts.Name)

				// Skip any PVs that aren't claimed by a PVC we are interested in
				if !strings.HasPrefix(pv.Spec.ClaimRef.Name, prefix) {
					continue
				}

				ordinal, err := strconv.ParseUint(pv.Spec.ClaimRef.Name[len(prefix):], 10, 16)
				if err != nil {
					return err
				}

				zone, ok := pv.Labels[ZoneLabel]
				if !ok {
					return fmt.Errorf("No zone on %s", pv.Name)
				}

				if _, ok := ret[zone]; !ok {
					ret[zone] = map[uint]bool{}
				}

				ret[zone][uint(ordinal)] = true

				return nil
			}

			return nil
		},
	); err != nil {
		return nil, err
	}

	return ret, nil
}
