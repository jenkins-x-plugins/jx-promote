package envctx

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/pkg/files"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/loadcreds"
	"github.com/jenkins-x/jx-helpers/pkg/stringhelpers"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/githelpers"
	"github.com/jenkins-x/jx-promote/pkg/versions"
	"github.com/jenkins-x/jx-promote/pkg/versionstream"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"sigs.k8s.io/yaml"

	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-promote/pkg/kube"
	"github.com/pkg/errors"
)

// LazyLoad lazy loads any missing values
func (e *EnvironmentContext) LazyLoad(jxClient versioned.Interface, ns string, gitter gits.Gitter, handles util.IOFileHandles) error {
	var err error
	if e.DevEnv == nil {
		e.DevEnv, err = kube.GetDevEnvironment(jxClient, ns)
		if err != nil {
			return errors.Wrapf(err, "failed to find dev environemnt in namespace %s", ns)
		}
	}
	if e.DevEnv == nil {
		return errors.Errorf("no dev environemnt in namespace %s", ns)
	}
	if e.Requirements == nil {
		e.Requirements, err = GetRequirementsConfigFromTeamSettings(&e.DevEnv.Spec.TeamSettings)
		if err != nil {
			return errors.Wrapf(err, "failed to read requirements from dev environment in namespace %s", ns)
		}
	}
	if e.Requirements == nil {
		return errors.Errorf("no Requirements in TeamSettings of dev environment in namespace %s", ns)
	}

	if e.VersionResolver == nil {
		// lets use the dev environment git repository
		url := e.DevEnv.Spec.Source.URL
		ref := "master"
		if url != "" {
			if e.GitUsername == "" || e.GitToken == "" {
				creds, err := loadcreds.LoadGitCredential()
				if err != nil {
					return errors.Wrapf(err, "failed to load git credentials")
				}

				gitInfo, err := giturl.ParseGitURL(url)
				if err != nil {
					return errors.Wrapf(err, "failed to parse git URL %s", url)
				}
				gitServerURL := gitInfo.HostURL()
				serverCreds := loadcreds.GetServerCredentials(creds, gitServerURL)

				if e.GitUsername == "" {
					e.GitUsername = serverCreds.Username
				}
				if e.GitToken == "" {
					e.GitToken = serverCreds.Password
				}
				if e.GitToken == "" {
					e.GitToken = serverCreds.Token
				}

				if e.GitUsername == "" {
					return errors.Errorf("could not find git user for git server %s", gitServerURL)
				}
				if e.GitToken == "" {
					return errors.Errorf("could not find git token for git server %s", gitServerURL)
				}
			}

			gitCloneURL, err := stringhelpers.URLSetUserPassword(url, e.GitUsername, e.GitToken)
			if err != nil {
				return errors.Wrapf(err, "failed to add user and token to git url %s", url)
			}

			cloneDir, err := githelpers.GitCloneToDir(gitter, gitCloneURL, ref, "")
			if err != nil {
				return errors.Wrapf(err, "failed to clone URL %s", gitCloneURL)
			}

			versionsDir := filepath.Join(cloneDir, "versionStream")
			exists, err := files.DirExists(versionsDir)
			if err != nil {
				return errors.Wrapf(err, "failed to check if version stream exists %s", versionsDir)
			}
			if !exists {
				return errors.Errorf("dev environment git repository %s does not have a versionStream dir", url)
			}

			e.VersionResolver = &versionstream.VersionResolver{
				VersionsDir: versionsDir,
			}
			log.Logger().Infof("using version stream from dev environment")
			return nil
		}

		log.Logger().Warnf("dev environment has no source URL configured")
		url = e.Requirements.VersionStream.URL
		ref = e.Requirements.VersionStream.Ref
		if ref == "" {
			ref = "master"
		}
		log.Logger().Infof("loading version stream URL %s ref %s", util.ColorInfo(url), util.ColorInfo(ref))

		e.VersionResolver, err = versions.CreateVersionResolver(url, ref, gitter)
		if err != nil {
			return errors.Wrapf(err, "failed to create VersionResolver")
		}
	}
	return nil
}

// GetRequirementsConfigFromTeamSettings reads the BootRequirements string from TeamSettings and unmarshals it
func GetRequirementsConfigFromTeamSettings(settings *v1.TeamSettings) (*config.RequirementsConfig, error) {
	if settings == nil {
		return nil, nil
	}

	// TeamSettings does not have a real value for BootRequirements, so this is probably not a boot cluster.
	if settings.BootRequirements == "" {
		return nil, nil
	}

	config := &config.RequirementsConfig{}
	data := []byte(settings.BootRequirements)
	err := yaml.Unmarshal(data, config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal requirements from team settings due to %s", err)
	}
	return config, nil
}
