package envctx

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/loadcreds"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/versionstream"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"sigs.k8s.io/yaml"

	v1 "github.com/jenkins-x/jx-api/v3/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/v3/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-api/v3/pkg/config"
	"github.com/pkg/errors"
)

// LazyLoad lazy loads any missing values
func (e *EnvironmentContext) LazyLoad(jxClient versioned.Interface, ns string, gitter gitclient.Interface) error {
	var err error
	if e.DevEnv == nil {
		e.DevEnv, err = jxenv.GetDevEnvironment(jxClient, ns)
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
		if url == "" {
			return errors.Errorf("environment %s does not have a source URL", e.DevEnv.Name)
		}
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

		cloneDir, err := gitclient.CloneToDir(gitter, gitCloneURL, "")
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

	requirements := &config.RequirementsConfig{}
	data := []byte(settings.BootRequirements)
	err := yaml.Unmarshal(data, requirements)
	if err != nil {
		return requirements, fmt.Errorf("failed to unmarshal requirements from team settings due to %s", err)
	}
	return requirements, nil
}
