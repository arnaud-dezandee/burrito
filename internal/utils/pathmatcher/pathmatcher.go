package pathmatcher

import (
	"path/filepath"
	"strings"

	"github.com/padok-team/burrito/internal/annotations"
)

func FilesHaveChanged(rootPath string, objectAnnotations map[string]string, changedFiles []string) bool {
	if len(changedFiles) == 0 {
		return true
	}

	for _, f := range changedFiles {
		f = ensureAbsPath(f)
		if strings.Contains(f, rootPath) {
			return true
		}
		if val, ok := objectAnnotations[annotations.AdditionnalTriggerPaths]; ok {
			for _, p := range strings.Split(val, ",") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") {
					p = ensureAbsPath(filepath.Clean(filepath.Join(rootPath, p)))
				} else {
					p = ensureAbsPath(filepath.Clean(p))
				}
				if strings.Contains(f, p) {
					return true
				}
			}
		}
	}

	return false
}

func ensureAbsPath(input string) string {
	if !filepath.IsAbs(input) {
		return string(filepath.Separator) + input
	}
	return input
}
