# redis-operator

[![CI](https://github.com/buildio/redis-operator/actions/workflows/ci.yml/badge.svg)](https://github.com/buildio/redis-operator/actions/workflows/ci.yml)
[![E2E Tests](https://github.com/buildio/redis-operator/actions/workflows/e2e.yml/badge.svg)](https://github.com/buildio/redis-operator/actions/workflows/e2e.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/buildio/redis-operator)](https://goreportcard.com/report/github.com/buildio/redis-operator)

Redis Operator creates/configures/manages redis-failovers atop Kubernetes.

This is a fork of `spotahome/redis-operator` → `Saremox/redis-operator` → `buildio/redis-operator`.

## What's New in v4.0.0

**Breaking Change: Instance Manager Required**

v4.0.0 makes the instance manager the default and only mode. Legacy exec probes are removed.

**Key changes:**
- **Sentinel disabled by default** - operator-managed failover is now the default
- Instance manager is always enabled (no opt-out)
- HTTP health probes (`/healthz`, `/readyz`) are now the only probe type
- Chart version aligned with operator version (4.0.0)

**Minimal configuration (operator-managed failover, no sentinel):**
```yaml
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 2
```

**With Redis Sentinel (opt-in):**
```yaml
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 2
  sentinel:
    enabled: true
    replicas: 3
```

## What's New in v1.7.0

**Sentinel-Free Architecture** ([#9](https://github.com/buildio/redis-operator/issues/9))

v1.7.0 introduced operator-managed failover as an alternative to Redis Sentinel, reducing pod overhead from 5 pods (2 Redis + 3 Sentinel) to just 2 pods (Redis only).

**How it works:**
- Operator monitors Redis pods and detects master failures
- On failure, promotes the replica with highest replication offset (minimizes data loss)
- Automatically reconfigures remaining replicas to follow new master
- Master Service (`rfrm-<name>`) endpoints update automatically via label selectors

**When to use:**
- Development/testing environments where you want fewer pods
- Cost-sensitive deployments where 3 Sentinel pods are overhead
- Simple HA setups where operator-managed failover is sufficient

**When to keep Sentinel:**
- Production environments requiring sub-second failover
- Complex topologies with multiple Redis clusters
- When you need Sentinel's pub/sub notifications

## What's New in v1.6.1

**Disable Service Links** ([#3](https://github.com/buildio/redis-operator/issues/3))

v1.6.1 sets `enableServiceLinks: false` on all pods to prevent startup failures in namespaces with many services. Kubernetes by default injects environment variables for every service in the namespace, which can exceed limits and cause pod failures.

## What's New in v1.6.0

**CNPG-style Instance Manager** ([#2](https://github.com/buildio/redis-operator/issues/2))

v1.6.0 introduces an optional instance manager that runs as PID 1 in Redis containers, following the [CloudNativePG model](https://cloudnative-pg.io/documentation/current/instance_manager/) which has proven reliable at scale.

**Features:**
- **RDB tempfile cleanup** - Automatically removes stale `temp-*.rdb` files on startup, preventing disk exhaustion during crash loops
- **Zombie process reaper** - Properly handles SIGCHLD for BGSAVE/BGREWRITEAOF child processes
- **Graceful shutdown** - Timeout escalation (SIGTERM → SIGKILL) for reliable shutdown

**Enabled by default in v4.0.0+** - no configuration needed.

### Roadmap

| Version | Features | Notes |
|---------|----------|-------|
| v1.6.0 | Instance Manager opt-in | `instanceManagerImage` field |
| v1.6.1 | Disable service links | Prevents startup failures in busy namespaces |
| v1.7.0 | Sentinel-free mode | `sentinel.enabled: false` |
| v4.0.0 | Instance Manager required | Current release - legacy probes removed, chart/operator versions aligned |

See [Issue #2](https://github.com/buildio/redis-operator/issues/2) for instance manager details and [Issue #9](https://github.com/buildio/redis-operator/issues/9) for sentinel-free architecture.

## Requirements

- Kubernetes: 1.21+
- Redis: 6+ (also supports Valkey 8)

Tested against Kubernetes 1.29, 1.30, 1.31, 1.32, 1.33, 1.34 and Redis 6, 7.

## Versioning

**Starting with 4.0.0, we no longer use 'v' prefix anywhere:**

| Git Tag | Chart Version | Image Tag |
|---------|---------------|-----------|
| 4.0.0 | 4.0.0 | 4.0.0 |

**Warning:** Previous releases used `v` prefix for git tags (e.g., `v1.7.0`). Starting with 4.0.0, git tags are bare version numbers (e.g., `4.0.0`).

If you don't specify `image.tag`, the chart automatically uses the appVersion.

## Quick Start

### Install from GitHub Container Registry (Recommended)

```bash
# Install CRD
kubectl apply --server-side -f https://raw.githubusercontent.com/buildio/redis-operator/main/manifests/databases.spotahome.com_redisfailovers.yaml

# Install operator (uses default image version matching chart)
helm upgrade --install redis-operator oci://ghcr.io/buildio/redis-operator/charts/redisoperator \
  --namespace redis-operator --create-namespace
```

No additional parameters required - the chart defaults to the correct image version.

### Install with Helm Repository

```bash
helm repo add redis-operator https://buildio.github.io/redis-operator
helm repo update
helm install redis-operator redis-operator/redis-operator
```

### Install with kubectl

```bash
REDIS_OPERATOR_VERSION=4.0.0
kubectl apply --server-side -f https://raw.githubusercontent.com/buildio/redis-operator/${REDIS_OPERATOR_VERSION}/manifests/databases.spotahome.com_redisfailovers.yaml
kubectl apply -f https://raw.githubusercontent.com/buildio/redis-operator/${REDIS_OPERATOR_VERSION}/example/operator/all-redis-operator-resources.yaml
```

### Install with Kustomize

```bash
# Default installation with RBAC, service account, resource limits
kustomize build github.com/buildio/redis-operator/manifests/kustomize/overlays/default?ref=4.0.0 | kubectl apply -f -

# Minimal installation
kustomize build github.com/buildio/redis-operator/manifests/kustomize/overlays/minimal?ref=4.0.0 | kubectl apply -f -

# Full installation with Prometheus ServiceMonitor
kustomize build github.com/buildio/redis-operator/manifests/kustomize/overlays/full?ref=4.0.0 | kubectl apply -f -
```

## Updating

### Update CRD

Helm only manages CRD creation on first install. To update the CRD:

```bash
REDIS_OPERATOR_VERSION=4.0.0
kubectl replace --server-side -f https://raw.githubusercontent.com/buildio/redis-operator/${REDIS_OPERATOR_VERSION}/manifests/databases.spotahome.com_redisfailovers.yaml
```

Then upgrade the operator:

```bash
helm upgrade redis-operator redis-operator/redis-operator
```

## Usage

### Create a Redis Failover

```bash
kubectl apply -f https://raw.githubusercontent.com/buildio/redis-operator/4.0.0/example/redisfailover/basic.yaml
```

This creates the following resources:
- `rfr-<NAME>`: Redis StatefulSet and ConfigMap
- `rfs-<NAME>`: Sentinel Deployment, ConfigMap, and Service

**Note:** The RedisFailover name must be ≤48 characters.

### Enable Instance Manager

To use the CNPG-style instance manager for improved reliability:

```yaml
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 2
  sentinel:
    replicas: 3
```

The instance manager is enabled by default:
1. An init container copies the `redis-instance` binary to a shared volume
2. The main container runs `redis-instance run` as PID 1
3. The instance manager performs cleanup and manages Redis as a child process


### Protect the master from cluster-autoscaler eviction

Setting `redis.preventMasterEviction: true` makes the operator annotate the current master pod with
`cluster-autoscaler.kubernetes.io/safe-to-evict: "false"` (and mark slaves `"true"`), so the
cluster-autoscaler will not drain the node running the master and trigger an avoidable failover. The
annotation follows the master as it moves. Defaults to `false` (no annotation is managed).

### Sentinel update strategy and PodDisruptionBudget

The sentinel `Deployment` update strategy can be overridden via `sentinel.strategy` (e.g. to set
`rollingUpdate.maxSurge`/`maxUnavailable`). This helps when required anti-affinity plus
`replicas == nodes` would otherwise deadlock the default rolling update:

```yaml
spec:
  sentinel:
    strategy:
      type: RollingUpdate
      rollingUpdate:
        maxSurge: 1
        maxUnavailable: 0
```

The `PodDisruptionBudget` `minAvailable` for each component defaults to `2` (or `1` when that
component's `replicas <= 2`). Override it per component with `redis.podDisruptionBudgetMinAvailable` /
`sentinel.podDisruptionBudgetMinAvailable` (an integer or percentage string such as `"60%"`).

### Persistence

The operator can add persistence to Redis data. By default, an `emptyDir` will be used, so the data is not saved.

To have persistence, a `PersistentVolumeClaim` usage is allowed. The full [PVC definition has to be added](example/redisfailover/persistent-storage.yaml) to the Redis Failover Spec under the `Storage` section.

**IMPORTANT**: By default, the persistent volume claims will be deleted when the Redis Failover is. If this is not the expected usage, a `keepAfterDeletion` flag can be added under the `storage` section of Redis. [An example is given](example/redisfailover/persistent-storage-no-pvc-deletion.yaml).

### NodeAffinity and Tolerations

You can use NodeAffinity and Tolerations to deploy Pods to isolated groups of Nodes. Examples are given for [node affinity](example/redisfailover/node-affinity.yaml), [pod anti-affinity](example/redisfailover/pod-anti-affinity.yaml) and [tolerations](example/redisfailover/tolerations.yaml).

## Topology Spread Constraints

You can use the `topologySpreadContraints` to ensure the pods of a type(redis or sentinel) are evenly distributed across zones/nodes. Examples are for using [topology spread constraints](example/redisfailover/topology-spread-contraints.yaml). Further document on how `topologySpreadConstraints` work could be found [here](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/).

### Custom configurations

It is possible to configure both Redis and Sentinel. This is done with the `customConfig` option inside their spec. It is a list of configurations and their values. This example is given in the [custom config example file](example/redisfailover/custom-config.yaml).

To have the ability of this configuration to be changed "on the fly," without the need of reload the redis/sentinel processes, the operator will apply them with calls to the redises/sentinels, using `config set` or `sentinel set mymaster` respectively. Because of this, **no changes on the configmaps** will appear regarding this custom configuration and the entries of `customConfig` from Redis spec will not be written on `redis.conf` file. To verify the actual Redis configuration use [`redis-cli CONFIG GET *`](https://redis.io/commands/config-get).

**Important**: in the Sentinel options, there are some "conversions" to be made:

- Configuration on the `sentinel.conf`: `sentinel down-after-milliseconds mymaster 2000`
- Configuration on the `configOptions`: `down-after-milliseconds 2000`

**Important 2**: do **NOT** change the options used for control the redis/sentinel such as `port`, `bind`, `dir`, etc.

### Managed maxmemory

With `redis.maxMemory` the operator sets `maxmemory` and `maxmemory-policy` from the redis container's memory limit, see the [maxmemory example file](example/redisfailover/maxmemory.yaml). It requires a memory limit of at least 64Mi; otherwise `maxmemory` is not managed and the reason is in the status message.

`maxmemory` is `percent` (default `75`) of the limit, keeping at least 32Mi free. `policy` defaults to `noeviction`.

| Limit | maxmemory |
|---|---|
| 64Mi | 32Mi |
| 96Mi | 64Mi |
| 128Mi | 96Mi |
| 1Gi | 768Mi |

Keys set in `customConfig` take precedence; `replica-ignore-maxmemory no` is rejected, as replicas would evict on their own, and running pods are set to `yes`. To migrate, update the CRD and the operator, add `maxMemory`, then remove `maxmemory` and `maxmemory-policy` from `customConfig`. Removing `maxMemory` leaves the running pods at their current values until they are recreated, which an in-place resize does not do; set them in `customConfig` to keep them.

`maxmemory` follows the smallest redis pod, as replicas hold the whole dataset and any of them can be promoted: a raised limit applies once every pod runs with it, a lowered one before the pods are replaced. `maxmemory` is only lowered below the memory in use under an `allkeys-*` policy, as `volatile-*` could evict every key with a TTL and still not fit; otherwise it is kept and the reason is in the status message. Until the data fits, the operator does not replace pods with the smaller limit. Pods recreated for other reasons, e.g. a node drain, get the smaller limit anyway.

For small instances, the default `client-output-buffer-limit` for `pubsub` (32mb) and `replica` (256mb) can exceed the free part of the limit; lower them with `customConfig`. Replicas buffer a whole `MULTI`/`EXEC` or `EVAL` before applying it, so one large batch can get a replica OOM-killed.

### In-place resize

On Kubernetes 1.33 or later, an update that only changes container cpu or memory resizes the redis pods in place instead of recreating them, so no data is reloaded and the master does not fail over. Pods are resized one at a time, replicas first. Lowering a memory limit in place needs Kubernetes 1.35. Set `redis.inPlaceResize: Disabled` to always recreate the pods.

A pod is still recreated when the update changes anything else, adds or removes requests or limits, or changes the pod's QoS class, when the node's kubelet does not support in-place resize, and when the kubelet reports the resize as infeasible, defers it or fails it for more than 5 minutes, or does not apply it within 5 minutes without reporting why. The operator needs `patch` on `pods/resize` and `get` on `controllerrevisions`, which the chart, the kustomize and the example manifests grant; without them the pods are recreated.

### Custom shutdown script

By default, a custom shutdown file is given. This file makes redis to `SAVE` it's data, and when Sentinel is enabled and redis is master, it'll call sentinel to ask for failover.

This behavior is configurable, creating a configmap and indicating to use it. An example about how to use this option can be found in the [shutdown example file](example/redisfailover/custom-shutdown.yaml).

**Important**: the configmap has to be in the same namespace. The configmap has to have a `shutdown.sh` data, containing the script.

### Custom SecurityContext

By default, Kubernetes will run containers as the user specified in the Dockerfile (or the root user if not specified); this is not always desirable.
If you need the containers to run as a specific user (or provide any other PodSecurityContext options), then you can specify a custom `securityContext` in the
`redisfailover` object. See the [SecurityContext example file](example/redisfailover/security-context.yaml) for an example. You can visit kubernetes documentation for detailed docs about [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)

A custom `securityContext` is merged on top of the operator defaults: fields you set win, and any field you leave unset keeps its default (e.g. setting only `runAsUser` no longer clears `fsGroup`/`runAsNonRoot`).

### Custom containerSecurityContext at container level

By default, Kubernetes will run containers with default docker capabilities, for example; this is not always desirable.
If you need the containers to run with specific capabilities or with read-only root file system (or provide any other securityContext options), then you can specify a custom `containerSecurityContext` in the
`redisfailover` object. See the [ContainerSecurityContext example file](example/redisfailover/container-security-context.yaml) for an example. Keys available under containerSecurityContext are detailed [here](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#securitycontext-v1-core)

A custom `containerSecurityContext` is merged on top of the operator defaults: fields you set win, and any field you leave unset keeps its default (e.g. the dropped `ALL` capabilities and `allowPrivilegeEscalation: false` are retained unless you override them).

### Custom command

By default, redis and sentinel will be called with the basic command, giving the configuration file:

- Redis: `redis-server /redis/redis.conf`
- Sentinel: `redis-server /redis/sentinel.conf --sentinel`

If necessary, this command can be changed with the `command` option inside redis/sentinel spec. An example can be found in the [custom command example file](example/redisfailover/custom-command.yaml).

### Custom environment variables

Extra environment variables can be injected into the redis and sentinel **main** containers via
`redis.env` / `sentinel.env` (standard Kubernetes `EnvVar` entries). The operator's own variables
(`REDIS_ADDR`, `REDIS_PORT`, `REDIS_USER`, `REDIS_PASSWORD`) always take precedence, so a
user-supplied variable that reuses one of those names cannot override it.

### Custom Priority Class
To use a custom Kubernetes [Priority Class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) for Redis and/or Sentinel pods, you can set the `priorityClassName` in the redis/sentinel spec, this attribute has no default and depends on the specific cluster configuration. **Note:** the operator doesn't create the referenced `Priority Class` resource.

### Custom Service Account
To use a custom Kubernetes [Service Account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/) for Redis and/or Sentinel pods, you can set the `serviceAccountName` in the redis/sentinel spec, if not specified the `default` Service Account will be used. **Note:** the operator doesn't create the referenced `Service Account` resource.

### Custom Pod Annotations
By default, no pod annotations will be applied to Redis nor Sentinel pods.

To apply custom pod Annotations, you can provide the `podAnnotations` option inside redis/sentinel spec. An example can be found in the [custom annotations example file](example/redisfailover/custom-annotations.yaml).
### Custom Service Annotations
By default, no service annotations will be applied to the Redis nor Sentinel services.

To apply custom service Annotations, you can provide the `serviceAnnotations` option inside redis/sentinel spec. An example can be found in the [custom annotations example file](example/redisfailover/custom-annotations.yaml).

### Control of label propagation.
By default, the operator will propagate all labels on the CRD down to the resources that it creates.
This can be problematic if the labels on the CRD are not fully under your own control, for example when they are being managed by a gitops operator.
Changes to those labels can fail on immutable resources such as PodDisruptionBudgets.
To control which labels the operator propagates to the resources it creates, you can modify the `labelWhitelist` option in the spec.

By default, specifying no whitelist or an empty whitelist will cause all labels to still be copied as not to break backwards compatibility.

Items in the array should be regular expressions, see [here](example/redisfailover/control-label-propagation.yaml) as an example of how they can be used and
[here](https://github.com/google/re2/wiki/Syntax) for a syntax reference.

The whitelist can also be used as a form of blacklist by specifying a regular expression that will not match any label.

NOTE: The operator will always add the labels it requires for operation to resources.  These are the following:
```
app.kubernetes.io/component
app.kubernetes.io/managed-by
app.kubernetes.io/name
app.kubernetes.io/part-of
redisfailovers.databases.spotahome.com/name
```


### ExtraVolumes and ExtraVolumeMounts

If the user chooses to have extra volumes creates and mounted, he could use the `extraVolumes` and `extraVolumeMounts`, in `spec.redis` of the CRD. This allows users to mount the extra configurations or secrets to be used. A typical use case for this might be
- Secrets that sidecars might use to back up of RDBs
- Extra users and their secrets and acls that could use the initContainers to create multiple users
- Extra Configurations that could merge on top of the existing configurations
- To pass failover scripts for addition for additional operations

```
---
apiVersion: v1
kind: Secret
metadata:
  name: foo
  namespace: exm
type: Opaque
stringData:
  password: MWYyZDFlMmU2N2Rm
---
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 2
  sentinel:
    replicas: 3
```

The instance manager is enabled by default:
1. An init container copies the `redis-instance` binary to a shared volume
2. The main container runs `redis-instance run` as PID 1
3. The instance manager performs cleanup and manages Redis as a child process

### Sentinel-Free Mode

For simpler deployments, disable Sentinel and let the operator manage failover:

```yaml
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 2
  sentinel:
    enabled: false
    failoverTimeout: "10s"  # Optional, defaults to 10s
```

**How failure detection works:**

The instance manager provides HTTP health endpoints (`/healthz`, `/readyz`) that enable:
- HTTP health probes (faster than exec probes)
- Immediate detection of Redis process crashes
- No process spawning overhead during health checks

This creates only:
- `rfr-<NAME>`: Redis StatefulSet (2 pods)
- `rfrm-<NAME>`: Master Service (points to current master via label selector)
- `rfrs-<NAME>`: Slave Service (points to replicas)

**No Sentinel pods are created.**

The operator handles failover by:
1. Detecting master failure via health checks
2. Selecting the replica with highest replication offset
3. Promoting it to master (`SLAVEOF NO ONE`)
4. Reconfiguring other replicas to follow the new master
5. Updating pod labels so Services route correctly

### Instance Manager CLI

The `redis-instance` binary provides the following commands:

```bash
# Run as instance manager (PID 1 mode)
redis-instance run --redis-conf /redis/redis.conf --data-dir /data --db-filename dump.rdb

# Standalone cleanup (removes stale RDB files)
redis-instance cleanup --data-dir /data --db-filename dump.rdb

# Dry-run cleanup (show what would be removed)
redis-instance cleanup --data-dir /data --dry-run
```

### Connection

**With Sentinel (default):**

Connect using a [Sentinel-ready client library](https://redis.io/topics/sentinel-clients):

```
url: rfs-<NAME>
port: 26379
master-name: mymaster
```

**Without Sentinel (`sentinel.enabled: false`):**

Connect directly to the master service:

```
url: rfrm-<NAME>
port: 6379
```

The master service automatically routes to the current master pod.

### Enable Authentication

```bash
kubectl create secret generic redis-auth --from-literal=password=your-password
```

```yaml
apiVersion: databases.spotahome.com/v1
kind: RedisFailover
metadata:
  name: my-redis
spec:
  redis:
    replicas: 3
  sentinel:
    replicas: 3
  auth:
    secretPath: redis-auth
```

Rotating the password (updating the `password` key of that same Secret in place), adding `auth.secretPath` or removing it is safe. The operator first switches every running Redis, and the Sentinels, to the new password in place with `CONFIG SET`, which keeps replication up, and then restarts the Redis pods one at a time onto the Secret. New connections need the new password right away.

Until a pod restarts, whatever reads the password from its environment keeps the old one: the exporter sidecar can't authenticate, the pre-stop `SAVE` fails, and so do custom probes using `$REDIS_PASSWORD`.

The operator knows the old password only from memory. If it restarted between the change and its next check, it can't switch the pods. The RedisFailover then reports `unable to apply the configured password`. Put the previous password back in the Secret, wait for the RedisFailover to become healthy, then change it again.

Enable persistent storage with a PVC:

```yaml
spec:
  redis:
    storage:
      persistentVolumeClaim:
        metadata:
          name: redis-data
        spec:
          accessModes: [ReadWriteOnce]
          resources:
            requests:
              storage: 10Gi
      keepAfterDeletion: true  # Optional: retain PVCs when RedisFailover is deleted
```

See [persistent-storage.yaml](example/redisfailover/persistent-storage.yaml) for a complete example.

### Custom Configuration

Configure Redis and Sentinel via `customConfig`:

```yaml
spec:
  redis:
    customConfig:
      - maxmemory 2gb
      - maxmemory-policy allkeys-lru
  sentinel:
    customConfig:
      - down-after-milliseconds 5000
```

**Note:** Configuration is applied via `CONFIG SET` at runtime. Do not modify control options like `port`, `bind`, or `dir`.

### Affinity and Tolerations

- [Node Affinity](example/redisfailover/node-affinity.yaml)
- [Pod Anti-Affinity](example/redisfailover/pod-anti-affinity.yaml)
- [Tolerations](example/redisfailover/tolerations.yaml)
- [Topology Spread Constraints](example/redisfailover/topology-spread-contraints.yaml)

### Security Context

- [Pod Security Context](example/redisfailover/security-context.yaml)
- [Container Security Context](example/redisfailover/container-security-context.yaml)

### Bootstrapping

Migrate from an existing Redis instance:

```yaml
spec:
  bootstrapNode:
    host: existing-redis.example.com
    port: "6379"
    allowSentinels: false  # Set true to also create Sentinels pointing to bootstrap node
```

See [bootstrapping.yaml](example/redisfailover/bootstrapping.yaml) for details.

## CI/CD

This project includes comprehensive GitHub Actions workflows:

| Workflow | Triggers | Description |
|----------|----------|-------------|
| CI | Push, PR | Build, lint, unit tests, integration tests, Docker build |
| E2E | PR | Full end-to-end tests in minikube cluster |
| Release | Tags | Multi-arch image build and push to GHCR |

### E2E Tests

The E2E workflow validates:
- Instance manager runs as PID 1
- RDB cleanup works on pod restart
- Redis remains functional after restart
- Sentinel-free mode: no Sentinel resources created
- Sentinel-free mode: operator-managed failover works

## Development

### Generate CRD

Requires [controller-gen](https://github.com/kubernetes-sigs/controller-tools) v0.20.0+ for Go 1.25+:

```bash
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
make generate-crd
```

### Run Tests

```bash
make ci-unit-test
make ci-integration-test
```

### Build Docker Image

```bash
make image
```

## Cleanup

### Remove Operator

```bash
helm uninstall redis-operator
kubectl delete crd redisfailovers.databases.spotahome.com
```

**Warning:** Deleting the CRD removes all RedisFailover resources and their managed objects.

### Remove Single RedisFailover

```bash
kubectl delete redisfailover <NAME>
```

All managed resources are automatically cleaned up via OwnerReferences.

## Docker Images

Images are published to GitHub Container Registry:

- **Operator & Instance Manager**: `ghcr.io/buildio/redis-operator`
- **Helm Chart**: `oci://ghcr.io/buildio/redis-operator/charts/redisoperator`

## Documentation

- [API Reference](docs/)
- [Examples](example/)
- [GoDoc](https://godoc.org/github.com/buildio/redis-operator)
