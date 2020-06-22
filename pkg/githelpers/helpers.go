package githelpers

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
)

var (
	pathSeparator = string(os.PathSeparator)
)

// AddAndCommitFiles add and commits files
func AddAndCommitFiles(gitter gits.Gitter, dir string, message string) (bool, error) {
	err := gitter.Add(dir, "*")
	if err != nil {
		return false, errors.Wrapf(err, "failed to add files to git")
	}
	changes, err := gitter.HasChanges(dir)
	if err != nil {
		if err != nil {
			return changes, errors.Wrapf(err, "failed to check if there are changes")
		}
	}
	if !changes {
		return changes, nil
	}
	err = gitter.CommitDir(dir, message)
	if err != nil {
		return changes, errors.Wrapf(err, "failed to git commit initial code changes")
	}
	return changes, nil
}

// CreateBranch creates a dynamic branch name and branch
func CreateBranch(gitter gits.Gitter, dir string) (string, error) {
	branchName := fmt.Sprintf("pr-%s", uuid.New().String())
	gitRef := branchName
	err := gitter.CreateBranch(dir, branchName)
	if err != nil {
		return branchName, errors.Wrapf(err, "create branch %s from %s", branchName, gitRef)
	}

	err = gitter.Checkout(dir, branchName)
	if err != nil {
		return branchName, errors.Wrapf(err, "checkout branch %s", branchName)
	}
	return branchName, nil
}

// GitCloneToTempDir clones the git repository to either the given directory or create a temporary
func GitCloneToTempDir(gitURL string, dir string) (string, error) {
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

	parentDir, name := filepath.Split(dir)
	parentDir = strings.TrimSuffix(parentDir, pathSeparator)

	err = GitCommand(parentDir, "git", "clone", gitURL, name)
	if err != nil {
		return dir, errors.Wrapf(err, "failed to ")
	}
	return dir, nil
}

// GitCommand runs a git command
func GitCommand(dir string, command string, args ...string) error {
	c := util.Command{
		Dir:  dir,
		Name: command,
		Args: args,
		In:   os.Stdin,
		Out:  os.Stdout,
		Err:  os.Stderr,
	}
	log.Logger().Infof("running command: %s", c.String())
	_, err := c.RunWithoutRetry()
	if err != nil {
		return errors.Wrap(err, "failed to run command")
	}
	return nil
}

// GitCloneToDir clones the git repository to a given or temporary directory
func GitCloneToDir(gitter gits.Gitter, gitURL string, gitRef string, dir string) (string, error) {
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

	return CheckoutRef(dir, gitter, gitURL, gitRef)
}

// CheckoutRef checks out the given reference
func CheckoutRef(dir string, gitter gits.Gitter, gitURL string, gitRef string) (string, error) {
	if gitRef != "" && gitRef != "master" && gitRef != "origin/master" && gitRef != "refs/heads/master" {
		_, err := checkoutRef(dir, gitRef, gitter)
		if err != nil {
			return dir, errors.Wrapf(err, "failed to checkout ref %s in repository %s", gitRef, gitURL)
		}
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

// AddUserTokenToURLIfRequired ensures we have a user and token in the given git URL
func AddUserTokenToURLIfRequired(gitURL, username, token string) (string, error) {
	u, err := url.Parse(gitURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse git URL %s", gitURL)
	}

	// lets check if we've already got a user and password
	if u.User != nil {
		user := u.User
		pwd, f := user.Password()
		if user.Username() != "" && pwd != "" && f {
			return gitURL, nil
		}
	}
	if username == "" {
		return "", fmt.Errorf("missing git username")
	}
	if token == "" {
		return "", fmt.Errorf("missing git token")
	}
	u.User = url.UserPassword(username, token)
	return u.String(), nil
}

// EnsureGitIgnoreContains ensures that the git ignore file in the given directory contains the given file
func EnsureGitIgnoreContains(gitter gits.Gitter, dir string, file string) error {
	path := filepath.Join(dir, ".gitignore")
	exists, err := util.FileExists(path)
	if err != nil {
		return errors.Wrapf(err, "failed checking exists %s", path)
	}
	source := ""
	if exists {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", path)
		}
		source = string(data)
		lines := strings.Split(source, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == file {
				return nil
			}
		}
	}
	source += "\n" + file + "\n"
	err = ioutil.WriteFile(path, []byte(source), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", path)
	}

	err = gitter.Add(dir, ".gitignore")
	if err != nil {
		return errors.Wrapf(err, "failed to add file %s to git", path)
	}
	err = gitter.CommitIfChanges(dir, fmt.Sprintf("fix: gitignore %s", file))
	if err != nil {
		return errors.Wrapf(err, "failed to commit file %s to git", path)
	}
	return nil
}
