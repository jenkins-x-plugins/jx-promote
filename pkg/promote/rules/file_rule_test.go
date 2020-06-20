package rules

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x/jx-promote/pkg/testhelpers"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileRules(t *testing.T) {
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

			dir := filepath.Join(tmpDir, name)

			src := filepath.Join("test_data", name)
			err = util.CopyDirOverwrite(src, dir)
			require.NoError(t, err, "could not copy source data in %s to %s", src, dir)

			config, _, err := promoteconfig.LoadPromote(dir, true)
			require.NoError(t, err, "failed to load config dir %s", dir)
			require.NotNil(t, config, "no project config found in dir %s", dir)
			require.NotNil(t, config.Spec.FileRule, "config.Spec.FileRule for %s", name)
			require.NotEmpty(t, config.Spec.FileRule.Path, "config.Spec.FileRule.Path for %s", name)
			require.NotEmpty(t, config.Spec.FileRule.InsertAfter, "config.Spec.FileRule.InsertAfter for %s", name)
			require.NotEmpty(t, config.Spec.FileRule.UpdateTemplate, "config.Spec.FileRule.UpdateTemplate for %s", name)
			require.NotEmpty(t, config.Spec.FileRule.CommandTemplate, "config.Spec.FileRule.CommandTemplate for %s", name)

			fileName := config.Spec.FileRule.Path
			target := filepath.Join(dir, fileName)
			assert.FileExists(t, target)

			r := &PromoteRule{
				Dir:       dir,
				Config:    *config,
				GitURL:    "https://github.com/myorg/myapp.git",
				Version:   "1.2.3",
				AppName:   "myapp",
				Namespace: ns,
			}

			err = FileRule(r)
			require.NoError(t, err, "failed to run FileRule at dir %s", dir)

			testhelpers.AssertTextFilesEqual(t, filepath.Join(src, fileName+".1.expected"), target, fileName)

			// now lets modify to new version
			r = &PromoteRule{
				Dir:       dir,
				Config:    *config,
				GitURL:    "https://github.com/myorg/myapp.git",
				Version:   "1.2.4",
				AppName:   "myapp",
				Namespace: ns,
			}

			err = FileRule(r)
			require.NoError(t, err, "failed to run FileRule at dir %s", dir)

			testhelpers.AssertTextFilesEqual(t, filepath.Join(src, fileName+".2.expected"), target, fileName)
		}
	}
}
