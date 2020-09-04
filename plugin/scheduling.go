package plugin

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

type Zone = string

type Node struct {
	Name string
	Zone Zone
}

func swap(slices []Zone, ordinal int, zone Zone) {
	for i, z := range slices {
		if zone == z {
			slices[i], slices[ordinal] = slices[ordinal], slices[i]
			return
		}
	}
}

func PodOrdinal(name string) int {
	spl := strings.Split(name, "-")
	o, err := strconv.Atoi(spl[len(spl)-1])
	if err != nil {
		return -1
	}
	return o
}

type ZonalTopology []Zone

func (n ZonalTopology) IdealZone(ordinal uint) Zone {
	return n[int(ordinal)%len(n)]
}

// BuildZonalTopology takes an existing distribution of a statefulset across nodes and zones
// and determines the ordinal to zone mapping. It defaults to alphabetical.
// For example an empty 3 node 3 zone cluster will return []{A, B, C}.
// Pod ordinals % 3 should be scheduled into the zone at that given index
// IE []{A, B, C}[0 % 3] == A, []{A, B, C}[1 % 3] == B
// It does it's best to determine what the order should be for existing distribution
// by "correcting" the alphabetical distribution.
func BuildZonalTopology(nodes []Node, vzd map[Zone]map[uint]bool) ZonalTopology {
	nodesByZone := map[Zone][]Node{}
	for _, node := range nodes {
		nodesByZone[node.Zone] = append(
			nodesByZone[node.Zone],
			node,
		)
	}

	slices := make([]Zone, 0, len(nodesByZone))
	for zone := range nodesByZone {
		slices = append(slices, zone)
	}

	// Build the default ordering heuristically.
	// This will have the largest impact on newly created clusters.
	// Consider the following layout
	// | A | B | C |
	// | 1 | 2 | 3 |
	// |   |   | 4 |
	// Zone C must be first to ensure that ordinal 3 get scheduled into it.
	sort.Slice(slices, func(i, j int) bool {
		if len(nodesByZone[slices[i]]) == len(nodesByZone[slices[j]]) {
			// Tie break alphabetically
			return slices[i] < slices[j]
		}

		// Otherwise order by zone size
		return len(nodesByZone[slices[i]]) > len(nodesByZone[slices[j]])
	})

	// Iterate over a clone. Otherwise swapping values may cause us to
	// skip a zone by accident and build an incorrect nodesByZoneribution.
	clone := append([]Zone{}, slices...)

	for _, zone := range clone {
		// Ignore empty zones
		if len(vzd[zone]) == 0 {
			continue
		}

		o := math.MaxInt32

		// Search for the lowest ordinal
		for ordinal := range vzd[zone] {
			if int(ordinal) < o {
				o = int(ordinal)
			}
		}

		// If no ordinal was not found skip checking
		if o == math.MaxInt32 {
			continue
		}

		// If alphabetical happened to be incorrect swap this zone
		// with the correct one
		// IE if we have []{A, B, C} and find that pod 0 should be in B
		// swap will return []{B, A, C}
		if slices[o%len(slices)] != zone {
			swap(slices, o%len(slices), zone)
		}
	}

	return slices
}
