package helm

import (
	"path/filepath"

	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// HelmRule uses a helm rule to create promote pull requests
func HelmRule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.ChartRule == nil {
		return errors.Errorf("no chartRule configured")
	}
	rule := config.Spec.ChartRule

	dir := r.Dir
	if rule.Path != "" {
		dir = filepath.Join(dir, rule.Path)
	}

	err := modifyChartFiles(r, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to modify chart files in dir %s", dir)
	}
	return nil
}

// modifyChartFiles modifies the chart files in the given directory using the given modify function
func modifyChartFiles(r *rules.PromoteRule, dir string) error {
	requirementsFile, err := helm.FindRequirementsFileName(dir)
	if err != nil {
		return err
	}

	exists, err := util.FileExists(requirementsFile)
	if err != nil {
		return errors.Wrapf(err, "failed to detect file %s", requirementsFile)
	}

	requirements := &helm.Requirements{}
	if exists {
		requirements, err = helm.LoadRequirementsFile(requirementsFile)
		if err != nil {
			return err
		}
	}

	chartFile, err := helm.FindChartFileName(dir)
	if err != nil {
		return err
	}

	chart, err := helm.LoadChartFile(chartFile)
	if err != nil {
		return err
	}

	err = modifyRequirements(r, requirements)
	if err != nil {
		return err
	}

	err = helm.SaveFile(requirementsFile, requirements)
	if err != nil {
		return err
	}

	err = helm.SaveFile(chartFile, chart)
	if err != nil {
		return err
	}
	return nil
}

func modifyRequirements(r *rules.PromoteRule, requirements *helm.Requirements) error {
	requirements.SetAppVersion(r.AppName, r.Version, r.HelmRepositoryURL, r.ChartAlias)
	return nil
}
