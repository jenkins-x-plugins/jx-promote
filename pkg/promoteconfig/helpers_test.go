package promoteconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPromoteNS = ""

func TestDiscoverPromoteConfigKpt(t *testing.T) {
	dir := filepath.Join("testdata", "custom", "cfg-root", "namespaces")
	cfg, fileName, err := promoteconfig.Discover(dir, testPromoteNS)
	require.NoError(t, err)
	require.NotEmpty(t, fileName, "no fileName returned")
	require.NotNil(t, cfg, "cfg not returned")

	require.NotNil(t, cfg.Spec.FileRule, "cfg.Spec.FileRule for %s", dir)
	require.NotEmpty(t, cfg.Spec.FileRule.Path, "cfg.Spec.FileRule.Path for %s", dir)
	require.NotEmpty(t, cfg.Spec.FileRule.InsertAfter, "cfg.Spec.FileRule.InsertAfter for %s", dir)
	require.NotEmpty(t, cfg.Spec.FileRule.UpdateTemplate, "cfg.Spec.FileRule.UpdateTemplate for %s", dir)
	require.NotEmpty(t, cfg.Spec.FileRule.CommandTemplate, "cfg.Spec.FileRule.CommandTemplate for %s", dir)

	t.Logf("loaded file %s with promote cfg %#v", fileName, cfg)
}

func TestDiscoverPromoteConfigHelm(t *testing.T) {
	dir := filepath.Join("testdata", "helm")
	cfg, fileName, err := promoteconfig.Discover(dir, testPromoteNS)
	require.NoError(t, err, "for dir %s", dir)
	require.NotNil(t, cfg, "config not returned for %s", dir)
	assert.Empty(t, fileName, "fileName for %s", dir)

	assert.NotNil(t, cfg.Spec.HelmRule, "cfg.Spec.HelmRule for %s", dir)
	assert.Equal(t, "env", cfg.Spec.HelmRule.Path, "cfg.Spec.HelmRule.Path for %s", dir)

	t.Logf("discovered config %#v for dir %s", cfg, dir)
}

func TestDiscoverPromoteConfigHelmfile(t *testing.T) {
	dir := filepath.Join("testdata", "helmfile")
	cfg, fileName, err := promoteconfig.Discover(dir, testPromoteNS)
	require.NoError(t, err, "for dir %s", dir)
	require.NotNil(t, cfg, "config not returned for %s", dir)
	assert.Empty(t, fileName, "fileName for %s", dir)

	assert.NotNil(t, cfg.Spec.HelmfileRule, "cfg.Spec.HelmfileRule for %s", dir)
	assert.Equal(t, "helmfile.yaml", cfg.Spec.HelmfileRule.Path, "cfg.Spec.HelmfileRule.Path for %s", dir)

	t.Logf("discovered config %#v for dir %s", cfg, dir)
}
