package helm

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/helmer"
)

// HelmRule uses a helm rule to create promote pull requests
func Rule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.HelmRule == nil {
		return fmt.Errorf("no helmRule configured")
	}
	rule := config.Spec.HelmRule

	dir := r.Dir
	if rule.Path != "" {
		dir = filepath.Join(dir, rule.Path)
	}

	err := modifyChartFiles(r, dir)
	if err != nil {
		return fmt.Errorf("failed to modify chart files in dir %s: %w", dir, err)
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
		return fmt.Errorf("failed to detect file %s: %w", requirementsFile, err)
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
