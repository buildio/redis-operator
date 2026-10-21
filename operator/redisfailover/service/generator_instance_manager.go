package service

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	redisfailoverv1 "github.com/saremox/redis-operator/api/redisfailover/v1"
)

// This file holds what this fork adds to the Redis pods of upstream
// Saremox/redis-operator: the instance manager. It is in a file of its own, so
// that a merge of upstream keeps the fork delta in one place.
//
// An init container copies the redis-instance binary from the operator image
// to an emptyDir volume, and the Redis container runs it as PID 1 with
// redis-server as its child. The manager removes the old RDB tempfiles before
// Redis starts, so this fork needs no rdb-tempfile-cleanup init container. It
// also serves the health endpoints that the liveness and the readiness probes
// use, so these probes start no process in the Redis container.
const (
	instanceManagerVolumeName  = "instance-manager"
	instanceManagerMountPath   = "/controller"
	instanceManagerBinaryName  = "redis-instance"
	instanceManagerHealthPort  = 8080
	instanceManagerHealthzPath = "/healthz"
	instanceManagerReadyzPath  = "/readyz"
)

// instanceManagerEnabled reports whether the Redis pods run the instance
// manager. Since 4.0.0 this fork always runs it, and it removed the probes
// that do not need it. The function stays, so that each place which depends on
// the manager names it.
func instanceManagerEnabled(_ *redisfailoverv1.RedisFailover) bool {
	return true
}

// getInstanceManagerImage returns the image that holds the redis-instance
// binary. Without spec.redis.instanceManagerImage, this is the operator image
// of the last release.
func getInstanceManagerImage(rf *redisfailoverv1.RedisFailover) string {
	if rf.Spec.Redis.InstanceManagerImage != "" {
		return rf.Spec.Redis.InstanceManagerImage
	}
	return redisfailoverv1.DefaultInstanceManagerImage
}

// instanceManagerPort is the container port of the health server.
func instanceManagerPort() corev1.ContainerPort {
	return corev1.ContainerPort{
		Name:          "health",
		ContainerPort: instanceManagerHealthPort,
		Protocol:      corev1.ProtocolTCP,
	}
}

// instanceManagerLivenessProbe asks the health server whether Redis answers.
// An HTTP probe starts no process, so it also works when the node has little
// memory left.
func instanceManagerLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: graceTime,
		TimeoutSeconds:      5,
		FailureThreshold:    6,
		PeriodSeconds:       15,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: instanceManagerHealthzPath,
				Port: intstr.FromInt32(instanceManagerHealthPort),
			},
		},
	}
}

// instanceManagerReadinessProbe asks the health server whether Redis can take
// the traffic: it does not load its dataset, it does not sync, and a replica
// has its link to the master.
func instanceManagerReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: graceTime,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
		FailureThreshold:    3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: instanceManagerReadyzPath,
				Port: intstr.FromInt32(instanceManagerHealthPort),
			},
		},
	}
}

// instanceManagerCommand starts the manager. The password is in no ConfigMap
// and in no pod command: the manager reads REDIS_PASSWORD from its
// environment and gives requirepass and masterauth to redis-server.
func instanceManagerCommand() []string {
	return []string{
		fmt.Sprintf("%s/%s", instanceManagerMountPath, instanceManagerBinaryName),
		"run",
		"--redis-conf", fmt.Sprintf("/redis/%s", redisConfigFileName),
	}
}

// instanceManagerVolume is the emptyDir volume that holds the binary between
// the init container and the Redis container.
func instanceManagerVolume() corev1.Volume {
	return corev1.Volume{
		Name: instanceManagerVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// instanceManagerVolumeMount mounts that volume in a container.
func instanceManagerVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      instanceManagerVolumeName,
		MountPath: instanceManagerMountPath,
	}
}

// createInstanceManagerInitContainer copies the redis-instance binary out of
// the operator image. The copy needs the cp utility, so the operator image is
// an Alpine image and not the distroless image that upstream uses.
func createInstanceManagerInitContainer(rf *redisfailoverv1.RedisFailover) corev1.Container {
	return corev1.Container{
		Name:            "instance-manager-init",
		Image:           getInstanceManagerImage(rf),
		ImagePullPolicy: pullPolicy(rf.Spec.Redis.ImagePullPolicy),
		Command: []string{
			"cp",
			fmt.Sprintf("/usr/local/bin/%s", instanceManagerBinaryName),
			fmt.Sprintf("%s/%s", instanceManagerMountPath, instanceManagerBinaryName),
		},
		VolumeMounts: []corev1.VolumeMount{instanceManagerVolumeMount()},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
}
