package rules_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/promote/rules"
	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/testhelpers"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleFactory(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err, "could not make a temp dir")

	t.Logf("creating tests at %s", tmpDir)

	sourceData := filepath.Join("test_data")
	files, err := ioutil.ReadDir(sourceData)
	assert.NoError(t, err)

	ns := "jx"
	for _, f := range files {
		if f.IsDir() {
			name := f.Name()
			if name == "jenkins-x-versions" {
				continue
			}

			dir := filepath.Join(tmpDir, name)

			src := filepath.Join("test_data", name)
			err = util.CopyDirOverwrite(src, dir)
			require.NoError(t, err, "could not copy source data in %s to %s", src, dir)

			cfg, _, err := promoteconfig.Discover(dir)
			require.NoError(t, err, "failed to load cfg dir %s", dir)
			require.NotNil(t, cfg, "no project cfg found in dir %s", dir)

			r := &rules.PromoteRule{
				TemplateContext: rules.TemplateContext{
					GitURL:    "https://github.com/myorg/myapp.git",
					Version:   "1.2.3",
					AppName:   "myapp",
					Namespace: ns,
				},
				Dir:           dir,
				Config:        *cfg,
				DevEnvContext: testhelpers.CreateTestDevEnvironmentContext(t, ns),
				ResolveChartRepositoryURL: func() (string, error) {
					return "http://chartmuseum-jx.34.78.195.22.nip.io", nil
				},
			}

			fn := rules.NewFunction(r)
			require.NotNil(t, fn, "failed to create RuleFunction at dir %s", dir)

			err = fn(r)
			require.NoError(t, err, "failed to invoke RuleFunction %v at dir %s", fn, dir)

			fileName := ruleFileName(cfg)
			target := filepath.Join(dir, fileName)
			assert.FileExists(t, target)

			testhelpers.AssertTextFilesEqual(t, filepath.Join(src, fileName+".1.expected"), target, fileName)

			// now lets modify to new version
			r.TemplateContext.Version = "1.2.4"

			err = fn(r)
			require.NoError(t, err, "failed to run FileRule at dir %s", dir)

			testhelpers.AssertTextFilesEqual(t, filepath.Join(src, fileName+".2.expected"), target, fileName)
		}
	}
}

func ruleFileName(cfg *v1alpha1.Promote) string {
	if cfg.Spec.AppsRule != nil {
		return cfg.Spec.AppsRule.Path
	}
	if cfg.Spec.ChartRule != nil {
		path := cfg.Spec.ChartRule.Path
		if path == "" {
			path = "."
		}
		return filepath.Join(path, "requirements.yaml")
	}
	if cfg.Spec.HelmfileRule != nil {
		return cfg.Spec.AppsRule.Path
	}
	return cfg.Spec.FileRule.Path
}
