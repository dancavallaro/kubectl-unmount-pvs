# kubectl-unmount-pvs

A `kubectl` plugin to unmount PersistentVolumes by scaling down workloads that use them.

## Installation

```shell
kubectl krew install unmount-pvs
```

## Usage

Unmount all PVs of a specific storage class:
```shell
kubectl unmount-pvs --storage-class=standard
```

Unmount all PVs in a namespace:
```shell
kubectl unmount-pvs --namespace=my-namespace
```

Combine filters:
```shell
kubectl unmount-pvs --namespace=my-namespace
```

Skip confirmation prompt:
```shell
kubectl unmount-pvs --storage-class=standard --yes
```
