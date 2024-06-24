package envctx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x-plugins/jx-gitops/pkg/variablefinders"
	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/loadcreds"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/versionstream"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"

	"github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
)

// LazyLoad lazy loads any missing values
func (e *EnvironmentContext) LazyLoad(gclient gitclient.Interface, jxClient versioned.Interface, ns string, gitter gitclient.Interface, dir string) error {
	var err error
	if e.DevEnv == nil {
		e.DevEnv, err = jxenv.GetDevEnvironment(jxClient, ns)
		if err != nil {
			return fmt.Errorf("failed to find dev environment in namespace %s: %w", ns, err)
		}
	}
	if e.DevEnv == nil {
		return fmt.Errorf("no dev environment in namespace %s", ns)
	}
	if e.Requirements == nil {
		e.Requirements, err = variablefinders.FindRequirements(gclient, jxClient, ns, dir, e.GitOwner, e.GitRepository)
		if err != nil {
			return fmt.Errorf("failed to load requirements from dev environment: %w", err)
		}

	}
	if e.Requirements == nil {
		return fmt.Errorf("no Requirements in TeamSettings of dev environment in namespace %s", ns)
	}

	// lets override the dev git URL if its changed in the requirements via the .jx/settings.yaml file
	devGitURL := requirements.EnvironmentGitURL(e.Requirements, "dev")
	if devGitURL != "" {
		e.DevEnv.Spec.Source.URL = devGitURL
	}

	if e.VersionResolver == nil {
		// lets use the dev environment git repository
		url := e.DevEnv.Spec.Source.URL
		if url == "" {
			return fmt.Errorf("environment %s does not have a source URL", e.DevEnv.Name)
		}
		if e.GitUsername == "" || e.GitToken == "" {
			creds, err := loadcreds.LoadGitCredential()
			if err != nil {
				return fmt.Errorf("failed to load git credentials: %w", err)
			}

			gitInfo, err := giturl.ParseGitURL(url)
			if err != nil {
				return fmt.Errorf("failed to parse git URL %s: %w", url, err)
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
				return fmt.Errorf("could not find git user for git server %s", gitServerURL)
			}
			if e.GitToken == "" {
				return fmt.Errorf("could not find git token for git server %s", gitServerURL)
			}
		}

		gitCloneURL, err := stringhelpers.URLSetUserPassword(url, e.GitUsername, e.GitToken)
		if err != nil {
			return fmt.Errorf("failed to add user and token to git url %s: %w", url, err)
		}

		cloneDir, err := gitclient.CloneToDir(gitter, gitCloneURL, "")
		if err != nil {
			return fmt.Errorf("failed to clone URL %s: %w", gitCloneURL, err)
		}

		versionsDir := filepath.Join(cloneDir, "versionStream")
		exists, err := files.DirExists(versionsDir)
		if err != nil {
			return fmt.Errorf("failed to check if version stream exists %s: %w", versionsDir, err)
		}
		if !exists {
			log.Logger().Warnf("dev environment git repository %s does not have a versionStream dir", url)
			err = os.MkdirAll(versionsDir, files.DefaultDirWritePermissions)
			if err != nil {
				return fmt.Errorf("failed to create version stream dir %s: %w", versionsDir, err)
			}
		}

		e.VersionResolver = &versionstream.VersionResolver{
			VersionsDir: versionsDir,
		}
		log.Logger().Infof("using version stream from dev environment")
		return nil
	}
	return nil
}
