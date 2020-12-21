package factory_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/testhelpers"
	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/jxtesthelpers"
	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-promote/pkg/rules/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleFactory(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err, "could not make a temp dir")

	t.Logf("creating tests at %s", tmpDir)

	sourceData := filepath.Join("test_data")
	fileSlice, err := ioutil.ReadDir(sourceData)
	assert.NoError(t, err)

	ns := "jx"
	testPromoteNS := "jx"
	for _, f := range fileSlice {
		if f.IsDir() {
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

			r := &rules.PromoteRule{
				TemplateContext: rules.TemplateContext{
					GitURL:            "https://github.com/myorg/myapp.git",
					Version:           "1.2.3",
					AppName:           "myapp",
					Namespace:         ns,
					HelmRepositoryURL: "http://chartmuseum-jx.34.78.195.22.nip.io",
				},
				Dir:           dir,
				Config:        *cfg,
				DevEnvContext: jxtesthelpers.CreateTestDevEnvironmentContext(t, ns),
			}

			fn := factory.NewFunction(r)
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

			if strings.HasPrefix(name, "helmfile-nested") {
				testhelpers.AssertTextFilesEqual(t, filepath.Join(src, "helmfile.yaml.expected"), filepath.Join(dir, "helmfile.yaml"), fileName)
			}
		}
	}
}

func ruleFileName(cfg *v1alpha1.Promote) string {
	if cfg.Spec.HelmRule != nil {
		path := cfg.Spec.HelmRule.Path
		if path == "" {
			path = "."
		}
		return filepath.Join(path, "requirements.yaml")
	}
	if cfg.Spec.HelmfileRule != nil {
		return cfg.Spec.HelmfileRule.Path
	}
	return cfg.Spec.FileRule.Path
}
