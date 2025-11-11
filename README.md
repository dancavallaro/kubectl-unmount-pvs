# kubectl-unmount

A `kubectl` plugin to unmount PersistentVolumes by scaling down workloads that use them.

## Installation

```shell
kubectl krew install unmount
```

## Usage

Unmount all PVs of a specific storage class:
```shell
kubectl unmount --storage-class=standard
```

Unmount all PVs in a namespace:
```shell
kubectl unmount --namespace=my-namespace
```

Combine filters:
```shell
kubectl unmount --namespace=my-namespace --storage-class=standard
```

Skip confirmation prompt:
```shell
kubectl unmount --storage-class=standard --yes
```

Dry run:
```shell
kubectl unmount --storage-class=standard --dry-run --yes
```
