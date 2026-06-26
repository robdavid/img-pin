# Docker image pinning

This project is a library and CLI tool for providing digested (hashed) image names for simply tagged images. The aim of the project to be a tool to mitigate supply chain attacks. It can:

* Provide a digest given a simple image registry, repository and tag (`--image` option).
* Patch in place image named used in a `Dockerfile`.
* Patch in place Kubernetes resources in YAML files (`--yaml` option). This includes best effort patching of k3s `HelmChart` resouces by adding options the `valuesContent` to pin image names where possible.
* Post patch Kubernetes resources before being applied to a cluster via `kubectl` (`--kube` option). This includes templating and post processing of `HelmChart` resources so that images names can be reliably be transformed. This pattern can provide the basis of a ArgoCD custom management plugin (CMP).
* Create a lock file of initially trusted digests to prevent drift.
* Provide the ability for downstream projects to build custom tools, via a simple plugin architecture, to bake in policy rules for image age, image name transformation, trusted repositories and more. It also allows custom YAML documents to provide Helm templating.
