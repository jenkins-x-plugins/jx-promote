package kpt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
)

// KptRule uses a jx-apps.yml file
func KptRule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.KptRule == nil {
		return errors.Errorf("no appsRule configured")
	}
	rule := config.Spec.KptRule

	gitURL := r.GitURL
	if gitURL == "" {
		return errors.Errorf("no GitURL for the app so cannot promote via kpt")
	}
	app := r.AppName
	if app == "" {
		return errors.Errorf("no AppName so cannot promote via kpt")
	}
	version := r.Version

	dir := r.Dir
	namespaceDir := dir
	kptPath := rule.Path
	if kptPath != "" {
		namespaceDir = filepath.Join(dir, kptPath)
	}

	appDir := filepath.Join(namespaceDir, app)
	// if the dir exists lets upgrade otherwise lets add it
	exists, err := util.DirExists(appDir)
	if err != nil {
		return errors.Wrapf(err, "failed to check if the app dir exists %s", appDir)
	}

	if version != "" && !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if version == "" {
		version = "master"
	}
	if exists {
		// lets upgrade the version via kpt
		args := []string{"pkg", "update", fmt.Sprintf("%s@%s", app, version), "--strategy=alpha-git-patch"}
		c := util.Command{
			Name: "kpt",
			Args: args,
			Dir:  namespaceDir,
		}
		log.Logger().Infof("running command: %s", c.String())
		_, err = c.RunWithoutRetry()
		if err != nil {
			return errors.Wrapf(err, "failed to update kpt app %s", app)
		}
	} else {
		if gitURL == "" {
			return errors.Errorf("no gitURL")
		}
		gitURL = strings.TrimSuffix(gitURL, "/")
		if !strings.HasSuffix(gitURL, ".git") {
			gitURL += ".git"
		}
		// lets add the path to the released kubernetes resources
		gitURL += fmt.Sprintf("/charts/%s/resources", app)
		args := []string{"pkg", "get", fmt.Sprintf("%s@%s", gitURL, version), app}
		c := util.Command{
			Name: "kpt",
			Args: args,
			Dir:  namespaceDir,
		}
		log.Logger().Infof("running command: %s", c.String())
		_, err = c.RunWithoutRetry()
		if err != nil {
			return errors.Wrapf(err, "failed to get the app %s via kpt", app)
		}
	}
	return nil
}
