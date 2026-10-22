package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	redisfailoverv1 "github.com/saremox/redis-operator/api/redisfailover/v1"
)

// withSentinel turns Sentinel on. sentinel.enabled defaults to false, so the
// RedisFailover that redisStatefulSetFor builds is sentinel-less.
func withSentinel(rf *redisfailoverv1.RedisFailover) {
	rf.Spec.Sentinel.Enabled = ptr.To(true)
}

// In Sentinel mode the hook asks Sentinel to fail the master over before it
// stops, and nothing else does this. It must stay.
func TestRedisKeepsThePreStopHookWithSentinel(t *testing.T) {
	ss := redisStatefulSetFor(t, withSentinel)

	require.NotEmpty(t, ss.Spec.Template.Spec.Containers)
	lifecycle := ss.Spec.Template.Spec.Containers[0].Lifecycle
	require.NotNil(t, lifecycle, "Sentinel mode needs the preStop hook to fail over")
	require.NotNil(t, lifecycle.PreStop)
	require.NotNil(t, lifecycle.PreStop.Exec)
	assert.Equal(t, []string{"/bin/sh", "/redis-shutdown/shutdown.sh"},
		lifecycle.PreStop.Exec.Command)
}

// Without Sentinel the script is only "redis-cli save". redis-server is PID 1
// and redis.conf has save points, so it saves on SIGTERM by itself. The hook
// adds nothing and costs the grace period, so the container gets none.
func TestRedisHasNoPreStopHookWithoutSentinel(t *testing.T) {
	ss := redisStatefulSetFor(t, nil)

	require.NotEmpty(t, ss.Spec.Template.Spec.Containers)
	assert.Nil(t, ss.Spec.Template.Spec.Containers[0].Lifecycle,
		"without Sentinel the hook only runs a synchronous SAVE")
}

// The ConfigMap and its mount stay in both modes, so that nothing else has to
// change and a user who runs the script by hand still finds it.
func TestRedisKeepsTheShutdownMountWithoutSentinel(t *testing.T) {
	ss := redisStatefulSetFor(t, nil)

	require.NotEmpty(t, ss.Spec.Template.Spec.Containers)
	var found bool
	for _, m := range ss.Spec.Template.Spec.Containers[0].VolumeMounts {
		if m.MountPath == "/redis-shutdown" {
			found = true
		}
	}
	assert.True(t, found, "the shutdown ConfigMap must stay mounted")
}
