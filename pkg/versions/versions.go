package versions

import (
	"strings"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx-promote/pkg/githelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
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
	versionsDir, err := githelpers.GitCloneToTempDir(git, versionRepository, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone URL %s", versionRepository)
	}
	if versionRef != "" && versionRef != "master" && versionRef != "origin/master" && versionRef != "refs/heads/master" {
		_, err = clone(versionsDir, versionRepository, versionRef, git)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout ref %s in repository %s", versionRef, versionRepository)
		}
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}

func clone(wrkDir string, versionRepository string, referenceName string, gitter gits.Gitter) (string, error) {
	if referenceName == "" || referenceName == "master" {
		referenceName = "refs/heads/master"
	} else if !strings.Contains(referenceName, "/") {
		if strings.HasPrefix(referenceName, "PR-") {
			prNumber := strings.TrimPrefix(referenceName, "PR-")

			log.Logger().Debugf("Cloning the Jenkins X versions repo %s with PR: %s to %s", util.ColorInfo(versionRepository), util.ColorInfo(referenceName), util.ColorInfo(wrkDir))
			return "", shallowCloneGitRepositoryToDir(wrkDir, versionRepository, prNumber, "", gitter)
		}
		log.Logger().Debugf("Cloning the Jenkins X versions repo %s with revision %s to %s", util.ColorInfo(versionRepository), util.ColorInfo(referenceName), util.ColorInfo(wrkDir))

		err := gitter.Clone(versionRepository, wrkDir)
		if err != nil {
			return "", errors.Wrapf(err, "failed to clone repository: %s to dir %s", versionRepository, wrkDir)
		}
		cmd := util.Command{
			Dir:  wrkDir,
			Name: "git",
			Args: []string{"fetch", "origin", referenceName},
		}
		_, err = cmd.RunWithoutRetry()
		if err != nil {
			return "", errors.Wrapf(err, "failed to git fetch origin %s for repo: %s in dir %s", referenceName, versionRepository, wrkDir)
		}
		isBranch, err := gits.RefIsBranch(wrkDir, referenceName, gitter)
		if err != nil {
			return "", err
		}
		if isBranch {
			err = gitter.Checkout(wrkDir, referenceName)
			if err != nil {
				return "", errors.Wrapf(err, "failed to checkout %s of repo: %s in dir %s", referenceName, versionRepository, wrkDir)
			}
			return "", nil
		}
		err = gitter.Checkout(wrkDir, "FETCH_HEAD")
		if err != nil {
			return "", errors.Wrapf(err, "failed to checkout FETCH_HEAD of repo: %s in dir %s", versionRepository, wrkDir)
		}
		return "", nil
	}
	log.Logger().Infof("Cloning the Jenkins X versions repo %s with ref %s to %s", util.ColorInfo(versionRepository), util.ColorInfo(referenceName), util.ColorInfo(wrkDir))
	// TODO: Change this to use gitter instead, but need to understand exactly what it's doing first.
	_, err := git.PlainClone(wrkDir, false, &git.CloneOptions{
		URL:           versionRepository,
		ReferenceName: plumbing.ReferenceName(referenceName),
		SingleBranch:  true,
		Progress:      nil,
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone reference: %s", referenceName)
	}
	return "", err
}

func shallowCloneGitRepositoryToDir(dir string, gitURL string, pullRequestNumber string, revision string, gitter gits.Gitter) error {
	if pullRequestNumber != "" {
		log.Logger().Infof("shallow cloning pull request %s of repository %s to temp dir %s", gitURL,
			pullRequestNumber, dir)
		err := gitter.ShallowClone(dir, gitURL, "", pullRequestNumber)
		if err != nil {
			return errors.Wrapf(err, "shallow cloning pull request %s of repository %s to temp dir %s\n", gitURL,
				pullRequestNumber, dir)
		}
	} else if revision != "" {
		log.Logger().Infof("shallow cloning revision %s of repository %s to temp dir %s", gitURL,
			revision, dir)
		err := gitter.ShallowClone(dir, gitURL, revision, "")
		if err != nil {
			return errors.Wrapf(err, "shallow cloning revision %s of repository %s to temp dir %s\n", gitURL,
				revision, dir)
		}
	} else {
		log.Logger().Infof("shallow cloning master of repository %s to temp dir %s", gitURL, dir)
		err := gitter.ShallowClone(dir, gitURL, "", "")
		if err != nil {
			return errors.Wrapf(err, "shallow cloning master of repository %s to temp dir %s\n", gitURL, dir)
		}
	}

	return nil
}
