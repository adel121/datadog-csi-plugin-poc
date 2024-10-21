# Datadog CSI Plugin POC

## Introduction

This repository contains a proof-of-concept of a Datadog CSI driver implementation.

The goal is to be able to mount hostpath dynamically onto user applicative pods without needing to mount volumes with hostpath types. Mounting such volumes needs to be avoided because it doesn't adhere to the minimal baseline [pod security standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) defined by kubernetes.

The CSI plugin takes care of mounting the hostpath using PVC (persistent volume claim) and CSI-based PV (persistent volume).

## Components

This project includes the following components:

- **CSI Driver**: The crucial component that implements the CSI interface, allowing Kubernetes to manage storage solutions dynamically.
- **Dockerfile**: Used to build the CSI driver's container image, ensuring compatibility across different architectures.
- **Deployment Manifests**: Kubernetes YAML files for deploying the CSI driver as a DaemonSet, along with the necessary RBAC configurations for authorization.

## Building

### Prerequisites

- Docker with Buildx support enabled.
- Go 1.22 or later.

### Build Instructions

1. **Prepare the Build Environment**:

   Create a new Docker Buildx builder instance to support multi-platform builds:

    ```sh
    docker buildx create --name mymultiarchbuilder --use
    docker buildx inspect --bootstrap
    ```

2. **Compile and Push the Container Image**:

    Build and push the multi-architecture image by running:

    ```bash
    docker buildx build --platform linux/amd64,linux/arm64 \
      -t <your-repo>/<your-csi-driver-image>:<tag> \
      --push \
      .
    ```
    
    Make sure to replace `<your-repo>`, `<your-csi-driver-image>`, and `<tag>` with your container registry details and desired image tag.

## Deploying

Deploy your CSI driver on Kubernetes to start leveraging dynamic storage provisioning capabilities.

Currently, and for the sake of simplicity, the CSI driver only includes a Node Server implementation which is deployed finally as a daemonset.

The CSI Node Server ensures the provisioning of the volume on the node and mounting the `/tmp/datadog` directory onto the pod mount point.

### Prerequisites

- A Kubernetes cluster.
- `kubectl`, configured to communicate with your cluster.

### Deployment Steps

To deploy the CSI driver on a kubernetes cluster, run the following command from the root directory of the repository:

`kubectl apply -f ./deploy`

### Testing

The demo folder contains files useful to test out this CSI plugin.

Follow the steps below to test it out:

#### Create the storage class

```
kubectl apply -f ./demo/storage-class.yaml
```

#### Create the pv and pvc

```
kubectl apply -f ./demo/pv.yaml
kubectl apply -f ./demo/pvc.yaml
```

#### Create the daemonset

```
kubectl apply -f ./demo/daemonset.yaml
```

You should see the daemonset pods running, with access to `/mount-test` directory, on which the CSI driver will have mounted the `/tmp/datadog/` directory from the host.
