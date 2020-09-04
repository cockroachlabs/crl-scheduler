# CRL Scheduler

The CRL Scheduler is the k8s scheduler compiled with a [custom scheduling plugin](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/).

Additional plugin examples can be [found here](https://github.com/kubernetes-sigs/scheduler-plugins).

It is currently compiled into the scheduler for kubernetes 1.16.

## The Issue

Deploying a statefulset across multiple availability zones may encounter some strange issues when changing the number of replicas.

Consider the following deployment of a statefulset with 4 replicas spread across 3 zones.

| A | B | C |
| - | - | - |
| 0 | 2 | 3 |
| 1 |   |   |

Suppose the number of replicas gets scaled down to 3

| A | B | C |
| - | - | - |
| 0 | 2 |   |
| 1 |   |   |

We've now lost the availability zone `C`.

Suppose we scale down to 1 replica and then back up to 4, again.

| A | B | C |
| - | - | - |
| 0 | 1 | 2 |
| 3 |   |   |

The scheduling of these pods is non-deterministic (Especially if nodes are being added and removed while the number of replicas change).

Pods 1, 2, and 3 are now unschedulable as their PVs (which are not removed by the statefulset controller) are still in the previous zones.

## The Solution

This scheduler plugin "locks" statefulset pods to specific zones bases on their ordinal, giving us deterministic scheduling at a zonal level.

## Building

```bash
# Build the docker image tagging as the current commit
make build

# Tag the image with todays date and push to GCR
make release
```
