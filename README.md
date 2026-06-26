# Docker image pinning

This project is a library and CLI tool for providing digested (hashed) image names for simply tagged images. The aim of the project is to be a tool to mitigate supply chain attacks by replacing mutable image tags (e.g., `ubuntu:24.04`) with immutable digest references (e.g., `ubuntu@sha256:abc123...`), patching them in-place across Dockerfiles and Kubernetes resource YAML.

## Install

```bash
go install github.com/robdavid/img-pin@latest
```

Or clone and build:

```bash
git clone <repo> && cd img-pin
go build -o img-pin .
```

## Usage

```
img-pin [flags] <file-or-image-args...>
```

### Resolve an image tag to a digest

Use `--image` (`-i`) to look up the current digest for one or more image tags:

```bash
img-pin -i ubuntu:24.04
img-pin -i nginx:1.25-alpine registry.example.com/app:latest
```

By default the output strips the tag. Use `--preserve-tags` (`-t`) to retain it:

```bash
img-pin -i -t ubuntu:24.04
# output: index.docker.io/library/ubuntu:24.04@sha256:abc...
```

### Patch Dockerfiles

Use `--dockerfile` (`-d`) to replace `FROM` image tags with digests in-place:

```bash
img-pin -d Dockerfile
img-pin -d path/to/Dockerfile another/Dockerfile
```

For templated Dockerfiles that don't parse cleanly, use `--lax` (`-l`) for regex-based parsing:

```bash
img-pin -d -l Dockerfile.tpl
```

Use `--verify` (`-v`) to check existing digests without modifying:

```bash
img-pin -v -d Dockerfile
```

### Patch Kubernetes YAML resources

Use `--yaml` (`-y`) to find and digest image references in Kubernetes resource files (supports `Deployment`, `StatefulSet`, `Job`, `CronJob`, `DaemonSet`, and k3s `HelmChart`):

```bash
img-pin -y deployment.yaml
img-pin -y k8s-manifest.yaml another.yaml
```

Control the YAML update strategy with `--update-method` (`-m`):

| Method   | Description |
|----------|-------------|
| `overwrite` | Replace the file completely |
| `patch`     | Apply a surgical diff-style patch |
| `sync`      | Preserve blank lines and comments (default) |

```bash
img-pin -y -m patch deployment.yaml
```

### Patch Helm charts via k3s HelmChart resources

The `--yaml` mode handles k3s `HelmChart` resources by adding digested image references to `valuesContent`. The tool does its best to find and transform image names referenced in the chart values.

### Post-process Kubernetes resources via kubectl

Use `--kube` (`-K`) to pipe Kubernetes manifests through img-pin before applying them. This expands Helm charts via `helm template` and digests all discovered images. It is designed as the basis for an ArgoCD Config Management Plugin (CMP):

```bash
kubectl apply -f <(img-pin -K manifest.yaml)
```

### Create and use lock files

Lock files record trusted digests so subsequent runs can skip registry calls and prevent silent drift.

Lock files are **ambiently honoured**: `--yaml` and `--kube` modes always look for a lock file matching the input file name (with `.lock.yaml` substituted for the extension). If no lock file exists, the tool proceeds normally against the registry. No explicit `-f` flag is needed — the lock file is picked up automatically when present alongside your manifests:

```bash
# deployment.lock.yaml is used automatically if it exists
img-pin -y deployment.yaml

# All *.yaml files get their respective lock files honoured
img-pin -K *.yaml
```

`.lock.yaml` files passed as positional arguments are automatically ignored (they are treated as lock files, not resource input).

Generate or update a lock file with `--lock` (`-k`):

```bash
img-pin -k -y deployment.yaml
# Creates/updates deployment.lock.yaml
```

The `--lock-file` (`-f`) flag is only required when reading from standard input (where no filename exists to derive a lock file name from):

```bash
cat deployment.yaml | img-pin -y -f images.lock.yaml -
```

Verify that existing digests still match the lock file:

```bash
img-pin -v -y deployment.yaml
```

### Update digests for moved tags

If a tag has moved to a new image (e.g., `ubuntu:24.04` now points to a different digest), you can update all references with `--update-digests` (`-u`). This implies `--preserve-tags` so the tag is retained next to the new digest:

```bash
img-pin -u -y deployment.yaml
```

### Policy-based image control

The tool provides a plugin architecture for custom image policies. Policies can enforce rules like minimum image age, rejection of `latest` tags, restricting to trusted registries, or transforming image names.

Built-in policies include:

| Policy               | Description |
|----------------------|-------------|
| `no-dockerhub`       | Reject images from Docker Hub |
| `reject-latest`      | Reject `:latest` tags |
| `min-age-<duration>` | Require minimum age (e.g., `min-age-720h`) |
| `skip`               | Skip images matching a pattern |
| `map-docker-to-aws`  | Rewrite Docker Hub references to AWS ECR |

Apply policies with `--policy` (`-p`):

```bash
img-pin -p no-dockerhub -p min-age-720h -y deployment.yaml
```

You can also define custom policies programmatically via the Go library for downstream projects to bake in their own rules for image age, name transformation, trusted repositories, and custom YAML Helm templating.

### Additional flags

| Flag | Short | Description |
|------|-------|-------------|
| `--batch` | `-b` | Continue processing on errors |
| `--counters` | `-c` | Print registry request counters |
| `--skip-auth` | `-a` | Skip private repos that require auth |
| `--skip-not-found` | `-N` | Skip images not found in registry |
| `--skip-post-verify` | `-P` | Skip post-verification pass |
| `--skip-v1-schema` | `-V` | Skip images with V1 schema |
| `--trim-multiline` | `-T` | Trim trailing whitespace from multiline YAML |
| `--log-level` | `-L` | Log level: `debug`, `info`, `warn`, `error` |
| `--crds` | `-C` | Dump CRDs from Helm charts |

## Example workflows

### CI pipeline gate

Prevent untrusted base images from entering your build:

```bash
img-pin --policy no-dockerhub --policy min-age-720h -i ubuntu:24.04
```

### Lock then deploy

Establish a baseline of trusted digests, then deploy everything against that lock:

```bash
# First run (needs registry access) — creates k8s/*.lock.yaml files
img-pin -k -y k8s/

# Deploy from lock (no registry calls) — lock files are honoured ambiently
img-pin -y k8s/
```

### ArgoCD CMP basis

See `--kube` mode above. The tool outputs fully digested manifests ready for cluster admission.

## Go library usage

The packages under `pkgs/` can be imported directly:

```go
import "github.com/robdavid/img-pin/pkgs/images"

digested, digest, created, err := images.Digest("ubuntu:24.04")
```

```go
import "github.com/robdavid/img-pin/pkgs/dockerfile"

count, verified, total, err := dockerfile.Patch("Dockerfile")
```

```go
import "github.com/robdavid/img-pin/pkgs/digester"

err := digester.CreateDigests("deployment.yaml")
```
