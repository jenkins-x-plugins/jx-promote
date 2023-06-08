package helmfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x-plugins/jx-promote/pkg/jxtesthelpers"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/helmfile"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveOCISchemeFromHelmfileRepositoriesDuringDefaultPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err, "could not make a temp dir")

	t.Logf("creating tests at %s", tmpDir)

	sourceData := "test_data"
	fileSlice, err := os.ReadDir(sourceData)

	assert.NoError(t, err)

	ns := "jx"
	testPromoteNS := "jx"
	for _, f := range fileSlice {
		if !f.IsDir() {
			continue
		}
		name := f.Name()
		if name == "jenkins-x-versions" {
			continue
		}

		dir := filepath.Join(tmpDir, name)

		src := filepath.Join("test_data", name)
		err = files.CopyDirOverwrite(src, dir)
		require.NoError(t, err, "could not copy source data in %s to %s", src, dir)

		cfg, _, err := promoteconfig.Discover(dir, testPromoteNS)
		require.NoError(t, err, "failed to load cfg dir %s", dir)
		require.NotNil(t, cfg, "no project cfg found in dir %s", dir)

		envctx := jxtesthelpers.CreateTestDevEnvironmentContext(t, ns)
		envctx.Requirements.Cluster.ChartKind = "oci"

		r := &rules.PromoteRule{
			TemplateContext: rules.TemplateContext{
				GitURL:            "https://github.com/myorg/myapp.git",
				Version:           "1.2.3",
				AppName:           "myapp",
				Namespace:         ns,
				HelmRepositoryURL: "oci://chartmuseum-jx.34.78.195.22.nip.io",
			},
			Dir:           dir,
			Config:        *cfg,
			DevEnvContext: envctx,
		}

		e := helmfile.Rule(r)
		require.Nil(t, e)
		testhelpers.AssertTextFilesEqual(t, filepath.Join(src, "helmfile.yaml.expected"), filepath.Join(dir, "helmfile.yaml"), "The OCI prefix has not been removed before adding it to the helmfile")

	}
}
