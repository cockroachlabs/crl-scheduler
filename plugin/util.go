package plugin

import (
	"k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

func Nodes(infoMap map[string]*nodeinfo.NodeInfo) []Node {
	nodes := make([]Node, 0, len(infoMap))
	for name, info := range infoMap {
		zone, ok := info.Node().Labels[ZoneLabel]
		if !ok {
			continue
		}

		nodes = append(nodes, Node{
			Name: name,
			Zone: zone,
		})
	}
	return nodes
}
