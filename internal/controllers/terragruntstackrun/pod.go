package terragruntstackrun

import (
	"context"
	"fmt"
	"strings"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/burrito/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action string

const (
	PlanAction  Action = "plan"
	ApplyAction Action = "apply"
)

func getDefaultLabels(run *configv1alpha1.TerragruntStackRun) map[string]string {
	return map[string]string{
		"burrito/component":  "runner",
		"burrito/managed-by": run.Name,
		"burrito/action":     string(run.Spec.Action),
	}
}

func (r *Reconciler) ensureCertificateAuthoritySecret(tenantNamespace, caSecretName string) error {
	secret := &corev1.Secret{}
	err := r.Client.Get(context.Background(), client.ObjectKey{Namespace: r.Config.Controller.MainNamespace, Name: caSecretName}, secret)
	if err != nil {
		return err
	}
	if _, ok := secret.Data["ca.crt"]; !ok {
		return fmt.Errorf("ca.crt not found in secret %s", caSecretName)
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: tenantNamespace},
		Data:       map[string][]byte{"ca.crt": secret.Data["ca.crt"]},
	}
	err = r.Client.Create(context.Background(), secret)
	if err != nil && apierrors.IsAlreadyExists(err) {
		err = r.Client.Update(context.Background(), secret)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func mountCA(podSpec *corev1.PodSpec, caSecretName, caName string) {
	volumeName := fmt.Sprintf("%s-cert", caName)
	mountPath := fmt.Sprintf("/etc/ssl/certs/%s.crt", caName)
	caFilename := fmt.Sprintf("%s.crt", caName)
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: caSecretName,
				Items:      []corev1.KeyToPath{{Key: "ca.crt", Path: caFilename}},
			},
		},
	})
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, corev1.VolumeMount{
		MountPath: mountPath,
		Name:      volumeName,
		SubPath:   caFilename,
	})
}

func (r *Reconciler) getPod(run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) corev1.Pod {
	defaultSpec := defaultPodSpec(r.Config, stack, run)
	if r.Config.Hermitcrab.Enabled {
		if err := r.ensureCertificateAuthoritySecret(stack.Namespace, r.Config.Hermitcrab.CertificateSecretName); err == nil {
			mountCA(&defaultSpec, r.Config.Hermitcrab.CertificateSecretName, "burrito-hermitcrab-ca")
			defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env,
				corev1.EnvVar{Name: "BURRITO_HERMITCRAB_ENABLED", Value: "true"},
				corev1.EnvVar{Name: "BURRITO_HERMITCRAB_URL", Value: fmt.Sprintf("https://burrito-hermitcrab.%s.svc.cluster.local/v1/providers/", r.Config.Controller.MainNamespace)},
			)
		}
	}
	if r.Config.Datastore.TLS {
		if err := r.ensureCertificateAuthoritySecret(stack.Namespace, r.Config.Datastore.CertificateSecretName); err == nil {
			mountCA(&defaultSpec, r.Config.Datastore.CertificateSecretName, "burrito-datastore-ca")
			defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env, corev1.EnvVar{Name: "BURRITO_DATASTORE_TLS", Value: "true"})
		}
	}
	defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env,
		corev1.EnvVar{Name: "BURRITO_RUNNER_ACTION", Value: run.Spec.Action},
		corev1.EnvVar{Name: "BURRITO_RUNNER_TARGET_KIND", Value: "stack"},
		corev1.EnvVar{Name: "BURRITO_RUNNER_STACK_NAME", Value: stack.Name},
		corev1.EnvVar{Name: "BURRITO_RUNNER_STACK_NAMESPACE", Value: stack.Namespace},
	)
	overrideSpec := configv1alpha1.GetOverrideRunnerSpecForStack(repository, stack)
	defaultSpec.Tolerations = overrideSpec.Tolerations
	defaultSpec.Affinity = overrideSpec.Affinity
	defaultSpec.Containers[0].Args = configv1alpha1.ChooseSlice(defaultSpec.Containers[0].Args, overrideSpec.Args)
	defaultSpec.Containers[0].Command = configv1alpha1.ChooseSlice(defaultSpec.Containers[0].Command, overrideSpec.Command)
	defaultSpec.NodeSelector = overrideSpec.NodeSelector
	defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env, overrideSpec.Env...)
	defaultSpec.InitContainers = overrideSpec.InitContainers
	defaultSpec.Volumes = append(defaultSpec.Volumes, overrideSpec.Volumes...)
	defaultSpec.Containers[0].VolumeMounts = append(defaultSpec.Containers[0].VolumeMounts, overrideSpec.VolumeMounts...)
	defaultSpec.Containers[0].Resources = overrideSpec.Resources
	defaultSpec.Containers[0].EnvFrom = append(defaultSpec.Containers[0].EnvFrom, overrideSpec.EnvFrom...)
	defaultSpec.ImagePullSecrets = append(defaultSpec.ImagePullSecrets, overrideSpec.ImagePullSecrets...)
	defaultSpec.Containers[0].ImagePullPolicy = overrideSpec.ImagePullPolicy
	if len(overrideSpec.ServiceAccountName) > 0 {
		defaultSpec.ServiceAccountName = overrideSpec.ServiceAccountName
	}
	if len(overrideSpec.Image) > 0 {
		defaultSpec.Containers[0].Image = overrideSpec.Image
	}
	if len(overrideSpec.ExtraInitArgs) > 0 {
		defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env, corev1.EnvVar{Name: "TF_CLI_ARGS_init", Value: strings.Join(overrideSpec.ExtraInitArgs, " ")})
	}
	if len(overrideSpec.ExtraPlanArgs) > 0 {
		defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env, corev1.EnvVar{Name: "TF_CLI_ARGS_plan", Value: strings.Join(overrideSpec.ExtraPlanArgs, " ")})
	}
	if len(overrideSpec.ExtraApplyArgs) > 0 {
		defaultSpec.Containers[0].Env = append(defaultSpec.Containers[0].Env, corev1.EnvVar{Name: "TF_CLI_ARGS_apply", Value: strings.Join(overrideSpec.ExtraApplyArgs, " ")})
	}
	pod := corev1.Pod{
		Spec: defaultSpec,
		ObjectMeta: metav1.ObjectMeta{
			Labels:      mergeMaps(overrideSpec.Metadata.Labels, getDefaultLabels(run)),
			Annotations: overrideSpec.Metadata.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: run.GetAPIVersion(),
				Kind:       run.GetKind(),
				Name:       run.Name,
				UID:        run.UID,
			}},
		},
	}
	pod.SetNamespace(stack.Namespace)
	pod.SetGenerateName(fmt.Sprintf("%s-%s-", stack.Name, run.Spec.Action))
	return pod
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func defaultPodSpec(config *config.Config, stack *configv1alpha1.TerragruntStack, run *configv1alpha1.TerragruntStackRun) corev1.PodSpec {
	return corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "ssh-known-hosts",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: config.Runner.SSHKnownHostsConfigMapName},
						Optional:             &[]bool{true}[0],
					},
				},
			},
			{
				Name: "burrito-token",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience:          "burrito",
							ExpirationSeconds: &[]int64{3600}[0],
							Path:              "token",
						}}},
					},
				},
			},
		},
		Containers: []corev1.Container{{
			Name:            "runner",
			Image:           fmt.Sprintf("%s:%s", config.Runner.Image.Repository, config.Runner.Image.Tag),
			ImagePullPolicy: corev1.PullPolicy(config.Runner.Image.PullPolicy),
			Args:            []string{"runner", "start"},
			Command:         []string{},
			Env: []corev1.EnvVar{
				{Name: "BURRITO_RUNNER_RUN", Value: run.Name},
				{Name: "BURRITO_RUNNER_SSH_KNOWN_HOSTS_CONFIG_MAP_NAME", Value: config.Runner.SSHKnownHostsConfigMapName},
				{Name: "BURRITO_RUNNER_RUNNER_BINARY_PATH", Value: config.Runner.RunnerBinaryPath},
				{Name: "BURRITO_RUNNER_REPOSITORY_PATH", Value: config.Runner.RepositoryPath},
				{Name: "BURRITO_DATASTORE_HOSTNAME", Value: config.Datastore.Hostname},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "ssh-known-hosts", MountPath: "/root/.ssh/known_hosts", SubPath: "known_hosts"},
				{Name: "burrito-token", MountPath: "/var/run/secrets/token/burrito", SubPath: "token"},
			},
		}},
		RestartPolicy: corev1.RestartPolicyNever,
	}
}
