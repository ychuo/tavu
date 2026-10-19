package views

import (
	"path/filepath"
	"strings"

	"tavu/internal/files"
)

// Standalone documents have no project root, so validate the full path before
// handing only its basename to the bounded file service. Canonicalize the
// parent (including macOS /var aliases), but retain a final symbolic link so
// the service's existing read-only and write protections still apply.
func standaloneFileSource(path string) (string, *files.Service, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	if standaloneGitMetadataPath(absolute) {
		return "", nil, files.ErrProtectedPath
	}
	service, err := files.New(filepath.Dir(absolute))
	if err != nil {
		return "", nil, err
	}
	absolute = filepath.Join(service.Root(), filepath.Base(absolute))
	if standaloneGitMetadataPath(absolute) {
		return "", nil, files.ErrProtectedPath
	}
	return absolute, service, nil
}

func standaloneGitMetadataPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}
