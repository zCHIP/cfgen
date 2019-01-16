#!/usr/bin/env bash
set -e

# Set docker to build on minikube
eval $(minikube docker-env)

# Build the image
docker build -t cfgen:latest .

# Run the deployment on minikube
kubectl apply -f resource-manifests/cfgen-deployment.yaml