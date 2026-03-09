package pathmatcher

import (
	"testing"

	"github.com/padok-team/burrito/internal/annotations"
)

func TestFilesHaveChanged(t *testing.T) {
	tests := []struct {
		name         string
		rootPath     string
		objectAnn    map[string]string
		changedFiles []string
		expected     bool
	}{
		{
			name:         "empty changed files triggers run",
			rootPath:     "live/prod",
			changedFiles: []string{},
			expected:     true,
		},
		{
			name:         "file under root path matches",
			rootPath:     "live/prod",
			changedFiles: []string{"live/prod/app/main.tf"},
			expected:     true,
		},
		{
			name:         "unrelated file does not match",
			rootPath:     "live/prod",
			changedFiles: []string{"modules/vpc/main.tf"},
			expected:     false,
		},
		{
			name:     "repository relative additional trigger path matches",
			rootPath: "live/prod",
			objectAnn: map[string]string{
				annotations.AdditionnalTriggerPaths: "modules",
			},
			changedFiles: []string{"modules/vpc/main.tf"},
			expected:     true,
		},
		{
			name:     "dot relative additional trigger path resolves from root path",
			rootPath: "live/prod",
			objectAnn: map[string]string{
				annotations.AdditionnalTriggerPaths: "./common",
			},
			changedFiles: []string{"live/prod/common/terragrunt.hcl"},
			expected:     true,
		},
		{
			name:     "parent relative additional trigger path resolves from root path",
			rootPath: "live/prod/app",
			objectAnn: map[string]string{
				annotations.AdditionnalTriggerPaths: "../shared",
			},
			changedFiles: []string{"live/prod/shared/providers.hcl"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilesHaveChanged(tt.rootPath, tt.objectAnn, tt.changedFiles)
			if got != tt.expected {
				t.Fatalf("FilesHaveChanged() = %v, want %v", got, tt.expected)
			}
		})
	}
}
