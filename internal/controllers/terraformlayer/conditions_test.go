package terraformlayer_test

import (
	"testing"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	controller "github.com/padok-team/burrito/internal/controllers/terraformlayer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLayerFilesHaveChanged(t *testing.T) {
	tests := []struct {
		name         string
		layer        configv1alpha1.TerraformLayer
		changedFiles []string
		expected     bool
	}{
		{
			name: "empty changed files returns true",
			layer: configv1alpha1.TerraformLayer{
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{},
			expected:     true,
		},
		{
			name: "file in layer path matches",
			layer: configv1alpha1.TerraformLayer{
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"environments/dev/main.tf"},
			expected:     true,
		},
		{
			name: "no files match layer path",
			layer: configv1alpha1.TerraformLayer{
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/prod",
				},
			},
			changedFiles: []string{"environments/dev/main.tf", "README.md"},
			expected:     false,
		},
		{
			name: "plain trigger path matches against repo root",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "modules",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"modules/vpc/main.tf"},
			expected:     true,
		},
		{
			name: "plain trigger path no match",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "modules",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"other/file.tf"},
			expected:     false,
		},
		{
			name: "multiple trigger paths comma separated",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "modules, shared",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"shared/common.tf"},
			expected:     true,
		},
		{
			name: "dot-relative trigger path resolves from layer path",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "./submodules",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"environments/dev/submodules/main.tf"},
			expected:     true,
		},
		{
			name: "dot-relative trigger path does not match repo root",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "./modules",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"modules/vpc/main.tf"},
			expected:     false,
		},
		{
			name: "parent relative trigger path resolves from layer path",
			layer: configv1alpha1.TerraformLayer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annotations.AdditionnalTriggerPaths: "../shared",
					},
				},
				Spec: configv1alpha1.TerraformLayerSpec{
					Path: "environments/dev",
				},
			},
			changedFiles: []string{"environments/shared/common.tf"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := controller.LayerFilesHaveChanged(tt.layer, tt.changedFiles)
			if got != tt.expected {
				t.Errorf("LayerFilesHaveChanged() = %v, want %v", got, tt.expected)
			}
		})
	}
}
