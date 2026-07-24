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

func (c *EnvironmentContext) LazyLoad(gitClient gitclient.Interface, jxClient versioned.Interface, ns string, gitter gitclient.Interface, dir string) error {
	err := c.loadDevEnv(jxClient, ns)
	if err != nil {
		return err
	}
	err = c.loadRequirements(gitClient, jxClient, ns, dir)
	if err != nil {
		return err
	}
	// requirements may override the dev environment git URL used for the version stream
	c.overrideDevGitURL()
	return c.loadVersionResolver(gitter)
}

// overrideDevGitURL overrides the dev environment git URL in place if the
// requirements (.jx/settings.yaml) specify a different one. The write-back is
// relied on by the cross-namespace git URL fallback in pkg/promote/promote.go.
func (c *EnvironmentContext) overrideDevGitURL() {
	if devGitURL := requirements.EnvironmentGitURL(c.Requirements, "dev"); devGitURL != "" {
		c.DevEnv.Spec.Source.URL = devGitURL
	}
}

// loadDevEnv loads the dev environment from the given namespace if not already loaded
func (c *EnvironmentContext) loadDevEnv(jxClient versioned.Interface, ns string) error {
	if c.DevEnv == nil {
		log.Logger().Infof("getting dev environment for namespace %s", ns)
		devEnv, err := jxenv.GetDevEnvironment(jxClient, ns)
		if err != nil {
			return fmt.Errorf("failed to find dev environment in namespace %s: %w", ns, err)
		}
		if devEnv == nil {
			return fmt.Errorf("no dev environment in namespace %s", ns)
		}
		log.Logger().Infof("found dev environment %s in namespace %s", devEnv.Name, ns)
		c.DevEnv = devEnv
	}
	return nil
}

// loadRequirements loads requirements from the dev environment
func (c *EnvironmentContext) loadRequirements(gclient gitclient.Interface, jxClient versioned.Interface, ns string, dir string) error {
	if c.Requirements == nil {
		log.Logger().Infof("loading requirements from dev environment in namespace %s", ns)
		devRequirements, err := variablefinders.FindRequirements(gclient, jxClient, ns, dir, c.GitOwner, c.GitRepository)
		if err != nil {
			return fmt.Errorf("failed to load requirements from dev environment: %w", err)
		}
		if devRequirements == nil {
			return fmt.Errorf("no requirements in dev environment in namespace %s", ns)
		}
		log.Logger().Infof("loaded requirements from dev environment in namespace %s", ns)
		c.Requirements = devRequirements
	}
	return nil
}

// loadVersionResolver resolves the version stream from the dev environment git
// repository if not already loaded
func (c *EnvironmentContext) loadVersionResolver(gitter gitclient.Interface) error {
	if c.VersionResolver != nil {
		return nil
	}

	url := c.DevEnv.Spec.Source.URL
	if url == "" {
		return fmt.Errorf("environment %s does not have a source URL", c.DevEnv.Name)
	}
	if err := c.resolveGitCredentials(url); err != nil {
		return err
	}

	cloneDir, err := c.cloneDevEnvRepo(gitter, url)
	if err != nil {
		return err
	}
	log.Logger().Infof("cloned dev environment git repository %s to %s", url, cloneDir)

	versionsDir, err := c.ensureVersionStreamDir(cloneDir, url)
	if err != nil {
		return fmt.Errorf("failed to ensure version stream directory in git repository %s: %w", url, err)
	}

	c.VersionResolver = &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}
	log.Logger().Infof("using version stream from dev environment")
	return nil
}

// resolveGitCredentials resolves Git Credentials for the given gitURL if not already set in the EnvironmentContext
func (c *EnvironmentContext) resolveGitCredentials(gitURL string) error {
	// credentials already supplied — nothing to resolve
	if c.GitUsername != "" && c.GitToken != "" {
		return nil
	}

	loadedCreds, err := loadcreds.LoadGitCredential()
	if err != nil {
		return fmt.Errorf("failed to load git credentials: %w", err)
	}

	gitInfo, err := giturl.ParseGitURL(gitURL)
	if err != nil {
		return fmt.Errorf("failed to parse git URL %s: %w", gitURL, err)
	}
	gitServerURL := gitInfo.HostURL()
	serverCreds := loadcreds.GetServerCredentials(loadedCreds, gitServerURL)

	if c.GitUsername == "" {
		c.GitUsername = serverCreds.Username
	}
	if c.GitToken == "" {
		c.GitToken = serverCreds.Password
	}
	if c.GitToken == "" {
		c.GitToken = serverCreds.Token
	}

	if c.GitUsername == "" {
		return fmt.Errorf("could not find git user for git server %s", gitServerURL)
	}
	if c.GitToken == "" {
		return fmt.Errorf("could not find git token for git server %s", gitServerURL)
	}

	log.Logger().Infof("resolved credentials for git server %s", gitServerURL)
	return nil
}

// cloneDevEnvRepo clones the dev environment git repository to a temporary directory and returns the path to the cloned directory.
// It sparsely and shallowly clones just the versionStream dir, falling back to a partial then full clone if the git server
// does not support sparse/partial checkout.
func (c *EnvironmentContext) cloneDevEnvRepo(gitter gitclient.Interface, gitURL string) (string, error) {
	gitCloneURL, err := stringhelpers.URLSetUserPassword(gitURL, c.GitUsername, c.GitToken)
	if err != nil {
		return "", fmt.Errorf("failed to add user and token to git url %s: %w", gitURL, err)
	}

	cloneDir, err := requirements.PartialCloneClusterRepo(gitter, gitCloneURL, true, "versionStream")
	if err != nil {
		return "", fmt.Errorf("failed to clone URL %s: %w", gitCloneURL, err)
	}
	if cloneDir == "" {
		return "", fmt.Errorf("failed to clone URL %s to dir %s", gitCloneURL, cloneDir)
	}

	return cloneDir, nil
}

// ensureVersionStreamDir checks the versionStream directory exists and creates it if not
func (c *EnvironmentContext) ensureVersionStreamDir(cloneDir, gitURL string) (string, error) {
	versionsDir := filepath.Join(cloneDir, "versionStream")
	exists, err := files.DirExists(versionsDir)
	if err != nil {
		return "", fmt.Errorf("failed to check if version stream exists %s: %w", versionsDir, err)
	}
	if !exists {
		log.Logger().Warnf("dev environment git repository %s does not have a versionStream dir", gitURL)
		err = os.MkdirAll(versionsDir, files.DefaultDirWritePermissions)
		if err != nil {
			return "", fmt.Errorf("failed to create version stream dir %s: %w", versionsDir, err)
		}
		if versionsDir == "" {
			return "", fmt.Errorf("failed to find version stream dir %s", versionsDir)
		}
	}
	return versionsDir, nil
}
