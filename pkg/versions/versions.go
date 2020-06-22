package versions

import (
	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx-promote/pkg/githelpers"
	"github.com/jenkins-x/jx-promote/pkg/versionstream"
	"github.com/jenkins-x/jx/v2/pkg/config"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
)

// GetDefaultVersionNumber returns the version number for the given kind and name using the default version stream
func GetDefaultVersionNumber(kind versionstream.VersionKind, name string) (string, error) {
	return GetVersionNumber(kind, name, config.DefaultVersionsURL, "master", nil, common.GetIOFileHandles(nil))
}

// GetVersionNumber returns the version number for the given kind and name or blank string if there is no locked version
func GetVersionNumber(kind versionstream.VersionKind, name, repo, gitRef string, git gits.Gitter, handles util.IOFileHandles) (string, error) {
	if git == nil {
		git = gits.NewGitCLI()
	}
	r, err := CreateVersionResolver(repo, gitRef, git)
	if err != nil {
		return "", err
	}
	return r.StableVersionNumber(kind, name)
}

// CreateVersionResolver creates a new VersionResolver service
func CreateVersionResolver(versionRepoURL string, versionRef string, gitter gits.Gitter) (*versionstream.VersionResolver, error) {
	versionsDir, err := githelpers.GitCloneToDir(gitter, versionRepoURL, versionRef, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone URL %s", versionRepoURL)
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}
