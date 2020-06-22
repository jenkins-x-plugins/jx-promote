package envctx

import (
	"fmt"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/versions"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx-promote/pkg/kube"
	v1 "github.com/jenkins-x/jx/v2/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/v2/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/v2/pkg/config"
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
		url := e.Requirements.VersionStream.URL
		ref := e.Requirements.VersionStream.Ref
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
