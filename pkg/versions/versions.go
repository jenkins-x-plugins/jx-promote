package versions

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
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
	r, err := CreateVersionResolver(repo, gitRef, git, handles)
	if err != nil {
		return "", err
	}
	return r.StableVersionNumber(kind, name)
}

// CreateVersionResolver creates a new VersionResolver service
func CreateVersionResolver(versionRepository string, versionRef string, git gits.Gitter, handles util.IOFileHandles) (*versionstream.VersionResolver, error) {
	versionsDir, err := GitCloneToDir(versionRepository, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone URL %s", versionRepository)
	}
	if versionRef != "" && versionRef != "master" && versionRef != "origin/master" && versionRef != "refs/heads/master" {
		_, err = checkoutRef(versionsDir, versionRef, git)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout ref %s in repository %s", versionRef, versionRepository)
		}
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}

// GitCloneToDir clones the git repository to a given or temporary directory
func GitCloneToDir(gitURL string, dir string) (string, error) {
	var err error
	if dir != "" {
		err = os.MkdirAll(dir, util.DefaultWritePermissions)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create directory %s", dir)
		}
	} else {
		dir, err = ioutil.TempDir("", "jx-promote-")
		if err != nil {
			return "", errors.Wrap(err, "failed to create temporary directory")
		}
	}

	log.Logger().Debugf("cloning %s to directory %s", util.ColorInfo(gitURL), util.ColorInfo(dir))

	_, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL:               gitURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone repository %s to directory: %s", gitURL, dir)
	}
	return dir, nil
}

func checkoutRef(wrkDir string, referenceName string, gitter gits.Gitter) (string, error) {
	if referenceName == "" || referenceName == "master" || referenceName == "refs/heads/master" {
		return "", nil
	}

	log.Logger().Infof("checking out ref %s", util.ColorInfo(referenceName))
	err := gitter.Checkout(wrkDir, referenceName)
	if err != nil {
		// lets see if its a tag
		if strings.HasPrefix(referenceName, "v") {
			referenceName = "tags/" + referenceName
			log.Logger().Infof("checking out ref %s", util.ColorInfo(referenceName))
			err = gitter.Checkout(wrkDir, referenceName)
		}
	}
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone ref %s", referenceName)
	}
	return "", nil
}
