package promoteconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromoteFind(t *testing.T) {
	dir := filepath.Join("test_data", "config-root", "namespaces")
	cfg, fileName, err := promoteconfig.LoadPromote(dir, true)
	require.NoError(t, err)
	require.NotEmpty(t, fileName, "no fileName returned")
	require.NotNil(t, cfg, "config not returned")

	assert.Equal(t, "namespaces/apps", cfg.Spec.KptPath, "spec.kptPath for dir %s", dir)
	assert.Equal(t, "myapps", cfg.Spec.Namespace, "spec.namespace for dir %s", dir)

	t.Logf("loaded file %s with promote config %#v", fileName, cfg)
}
