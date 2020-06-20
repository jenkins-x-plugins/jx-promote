package rules

import (
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/pkg/errors"
)

// AppsRule uses a jx-apps.yml file
func AppsRule(r *PromoteRule) error {
	config := r.Config
	if config.Spec.AppsRule == nil {
		return errors.Errorf("no appsRule configured")
	}
	rule := config.Spec.AppsRule
	err := modifyAppsFile(r, r.Dir, rule.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to modify chart files in dir %s", r.Dir)
	}
	return nil
}

// ModifyAppsFile modifies the 'jx-apps.yml' file to add/update/remove apps
func modifyAppsFile(r *PromoteRule, dir string, file string) error {
	appsConfig, fileName, err := config.LoadAppConfig(dir)
	if fileName == "" {
		// if we don't have a `jx-apps.yml` then just return immediately
		return nil
	}
	if err != nil {
		return err
	}
	err = modifyApps(r, appsConfig)
	if err != nil {
		return err
	}

	err = appsConfig.SaveConfig(fileName)
	if err != nil {
		return err
	}
	return nil
}

func modifyApps(r *PromoteRule, appsConfig *config.AppConfig) error {
	if r.ResolveChartRepositoryURL == nil {
		return errors.Errorf("no ResolveChartRepositoryURL()")
	}
	repositoryURL, err := r.ResolveChartRepositoryURL()
	if err != nil {
		return errors.Wrap(err, "failed to resolve chart museum URL")
	}

	if r.DevEnvContext == nil {
		return errors.Errorf("no devEnvContext")
	}
	app := r.AppName
	version := r.Version
	details, err := r.DevEnvContext.ChartDetails(app, repositoryURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get chart details for %s repo %s", app, repositoryURL)
	}
	details.DefaultPrefix(appsConfig, "dev")

	for i := range appsConfig.Apps {
		appConfig := &appsConfig.Apps[i]
		if appConfig.Name == app || appConfig.Name == details.Name {
			appConfig.Version = version
			return nil
		}
	}
	appsConfig.Apps = append(appsConfig.Apps, config.App{
		Name:    details.Name,
		Version: version,
	})
	return nil
}
