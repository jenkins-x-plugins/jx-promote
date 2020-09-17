package helm

import (
	"path/filepath"

	"github.com/jenkins-x/jx-helpers/pkg/files"
	"github.com/jenkins-x/jx-helpers/pkg/helmer"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/pkg/errors"
)

// HelmRule uses a helm rule to create promote pull requests
func Rule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.HelmRule == nil {
		return errors.Errorf("no helmRule configured")
	}
	rule := config.Spec.HelmRule

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
	requirementsFile, err := helmer.FindRequirementsFileName(dir)
	if err != nil {
		return err
	}

	exists, err := files.FileExists(requirementsFile)
	if err != nil {
		return errors.Wrapf(err, "failed to detect file %s", requirementsFile)
	}

	requirements := &helmer.Requirements{}
	if exists {
		requirements, err = helmer.LoadRequirementsFile(requirementsFile)
		if err != nil {
			return err
		}
	}

	chartFile, err := helmer.FindChartFileName(dir)
	if err != nil {
		return err
	}

	chart, err := helmer.LoadChartFile(chartFile)
	if err != nil {
		return err
	}

	err = modifyRequirements(r, requirements)
	if err != nil {
		return err
	}

	err = helmer.SaveFile(requirementsFile, requirements)
	if err != nil {
		return err
	}

	err = helmer.SaveFile(chartFile, chart)
	if err != nil {
		return err
	}
	return nil
}

func modifyRequirements(r *rules.PromoteRule, requirements *helmer.Requirements) error {
	requirements.SetAppVersion(r.AppName, r.Version, r.HelmRepositoryURL, r.ChartAlias)
	return nil
}
