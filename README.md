# cfgen

A simple library with CLI and k8s cluster-based applications to generate custom nginx configurations for k8s services.

## cfgencli

CLI tool is used to generate configs being out of k8s clusters. It uses user's `.kube/config` by default.

### Installation

You need to have Docker 18.X installed.

```bash
# Build the image
docker build -t cfgen:latest .
```

```bash
# Run the CLI app. By default, it generates configs to system stdout for "default" k8s namespace
docker run --entrypoint "/go/bin/cfgencli" cfgen:latest
```

```bash
# Run the CLI app with custom .kube/config config location, config output dit and specific namespace
docker run --entrypoint "/go/bin/cfgencli" cfgen:latest -kube-config /path/to/conf -namespace devland -output-path /path/to/save/cfgs/
```

## cfgensvc

Web application designed to be run in k8s clusters. Listen to k8s events and manages nginx configuration files if a new service added or deleted.

### Installation

You need to have `kubectl` installed. There are prepared k8s resource manifests for installation.

```bash
# Create service account (SA)
kubectl create -f resource-manifests/cfgen-sa.yaml

```

```bash
# Create k8s RBAC role
kubectl create -f resource-manifests/cfgen-role.yaml

```

```bash
# Bind SA to k8s RBAC role
kubectl create -f resource-manifests/cfgen-sa-rolebinding.yaml

```

```bash
# Create the deployment for cfgen application
kubectl create resource-manifests/cfgen-deployment.yaml
```