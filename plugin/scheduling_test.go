package plugin_test

import (
	"testing"

	"github.com/cockroachlabs/crl-scheduler/plugin"
	"github.com/stretchr/testify/require"
)

const (
	ZoneA = plugin.Zone("A")
	ZoneB = plugin.Zone("B")
	ZoneC = plugin.Zone("C")
	ZoneD = plugin.Zone("D")
)

func TestPodOrdinal(t *testing.T) {
	require.Equal(t, 0, plugin.PodOrdinal("cockroachdb-0"))
	require.Equal(t, 1, plugin.PodOrdinal("cockroachdb-1"))
	require.Equal(t, 20, plugin.PodOrdinal("cockroachdb-20"))
	require.Equal(t, 31, plugin.PodOrdinal("coc-kroach-db-31"))
	require.Equal(t, -1, plugin.PodOrdinal("coc-kroach-db-"))
	require.Equal(t, -1, plugin.PodOrdinal(""))
	require.Equal(t, -1, plugin.PodOrdinal("cockroach"))
}

func TestBuildZonalTopology(t *testing.T) {
	testCases := []struct {
		Nodes   []plugin.Node
		Volumes map[plugin.Zone]map[uint]bool
		Out     plugin.ZonalTopology
	}{
		{
			[]plugin.Node{},
			map[plugin.Zone]map[uint]bool{},
			[]plugin.Zone{},
		}, {
			[]plugin.Node{
				{Name: "foo", Zone: ZoneA},
				{Name: "bar", Zone: ZoneB},
				{Name: "baz", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true},
				ZoneB: {1: true},
				ZoneC: {2: true},
			},
			[]plugin.Zone{ZoneA, ZoneB, ZoneC},
		}, {
			[]plugin.Node{
				{Name: "foo", Zone: ZoneA},
				{Name: "bar", Zone: ZoneB},
				{Name: "baz", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true},
				ZoneC: {1: true},
			},
			[]plugin.Zone{ZoneA, ZoneC, ZoneB},
		}, {
			[]plugin.Node{
				{Name: "1", Zone: ZoneA},
				{Name: "2", Zone: ZoneA},
				{Name: "3", Zone: ZoneB},
				{Name: "4", Zone: ZoneB},
				{Name: "5", Zone: ZoneC},
				{Name: "6", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true, 1: true},
				ZoneB: {2: true, 4: true},
				ZoneC: {3: true, 5: true},
			},
			[]plugin.Zone{ZoneC, ZoneA, ZoneB},
		}, {
			[]plugin.Node{
				{Name: "1", Zone: ZoneA},
				{Name: "2", Zone: ZoneB},
				{Name: "3", Zone: ZoneC},
				{Name: "4", Zone: ZoneD},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true},
				ZoneB: {1: true},
				ZoneC: {2: true},
				ZoneD: {3: true},
			},
			[]plugin.Zone{ZoneA, ZoneB, ZoneC, ZoneD},
		}, {
			[]plugin.Node{
				{Name: "A-1", Zone: ZoneA},
				{Name: "A-2", Zone: ZoneA},
				{Name: "A-3", Zone: ZoneA},
				{Name: "B-1", Zone: ZoneB},
				{Name: "C-1", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true, 1: true, 2: true},
			},
			[]plugin.Zone{ZoneA, ZoneB, ZoneC},
		}, {
			[]plugin.Node{
				{Name: "gke-crdb-pool-eb103251-s16e", Zone: ZoneB},
				{Name: "gke-crdb-pool-ec8991a4-f0zd", Zone: ZoneC},
				{Name: "gke-crdb-pool-ab6423ef-cw51", Zone: ZoneD},
			},
			map[plugin.Zone]map[uint]bool{
				ZoneB: {2: true},
				ZoneD: {1: true},
			},
			[]plugin.Zone{ZoneC, ZoneD, ZoneB},
		}, {
			[]plugin.Node{
				{Name: "A-1", Zone: ZoneA},
				{Name: "B-1", Zone: ZoneB},
				{Name: "B-2", Zone: ZoneB},
				{Name: "C-1", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{},
			[]plugin.Zone{ZoneB, ZoneA, ZoneC},
		}, {
			[]plugin.Node{
				{Name: "A-1", Zone: ZoneA},
				{Name: "B-1", Zone: ZoneB},
				{Name: "B-2", Zone: ZoneB},
				{Name: "C-1", Zone: ZoneC},
				{Name: "C-2", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{},
			[]plugin.Zone{ZoneB, ZoneC, ZoneA},
		}, {
			[]plugin.Node{
				{Name: "A-1", Zone: ZoneA},
				{Name: "A-2", Zone: ZoneA},
				{Name: "B-1", Zone: ZoneB},
				{Name: "B-2", Zone: ZoneB},
				{Name: "C-1", Zone: ZoneC},
				{Name: "C-2", Zone: ZoneC},
			},
			map[plugin.Zone]map[uint]bool{},
			[]plugin.Zone{ZoneA, ZoneB, ZoneC},
		}, {
			[]plugin.Node{
				{Name: "A-1", Zone: ZoneA},
				{Name: "A-2", Zone: ZoneA},
				{Name: "A-3", Zone: ZoneA},
				{Name: "B-1", Zone: ZoneB},
				{Name: "B-2", Zone: ZoneB},
				{Name: "B-3", Zone: ZoneB},
				{Name: "C-1", Zone: ZoneC},
				{Name: "C-2", Zone: ZoneC},
				{Name: "C-3", Zone: ZoneC},
			},
			// TODO(chrisseto): This is less than ideal.
			// The correct layout should be C, B, A
			// this isn't a very realistic case, so the time to correct is
			// not currently worth it.
			map[plugin.Zone]map[uint]bool{
				ZoneA: {0: true, 5: true, 8: true},
				ZoneB: {1: true, 4: true, 7: true},
				ZoneC: {2: true, 3: true, 6: true},
			},
			[]plugin.Zone{ZoneA, ZoneB, ZoneC},
		},
	}

	for _, tc := range testCases {
		require.Equalf(t, tc.Out, plugin.BuildZonalTopology(tc.Nodes, tc.Volumes), "expected %#v to have slices %#v", tc.Nodes, tc.Out)
	}
}
