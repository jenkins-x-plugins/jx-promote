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

func TestMakefileRules(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err, "could not make a temp dir")

	err = util.CopyDirOverwrite("test_data", dir)
	require.NoError(t, err, "could not copy source data to %s", dir)

	makefile := filepath.Join(dir, "Makefile")
	assert.FileExists(t, makefile)

	expectedFile := filepath.Join("test_data", "Makefile.1.expected")
	assert.FileExistsf(t, expectedFile, "should have expected file")

	config, _, err := promoteconfig.LoadPromote(dir, true)
	require.NoError(t, err, "failed to load config dir %s", dir)
	require.NotNil(t, config, "no project config found in dir %s", dir)
	require.NotNil(t, config.Spec.MakefileRule, "config.Spec.MakefileRule")
	require.NotEmpty(t, config.Spec.MakefileRule.InsertAfterPrefix, "config.Spec.MakefileRule.InsertAfterPrefix")
	require.NotEmpty(t, config.Spec.MakefileRule.UpdatePrefixTemplate, "config.Spec.MakefileRule.UpdatePrefixTemplate")
	require.NotEmpty(t, config.Spec.MakefileRule.CommandTemplate, "config.Spec.MakefileRule.CommandTemplate")

	r := &PromoteRule{
		Dir:     dir,
		Config:  *config,
		GitURL:  "https://github.com/myorg/myapp.git",
		Version: "1.2.3",
		AppName: "myapp",
	}

	err = MakefileRule(r)
	require.NoError(t, err, "failed to run MakefileRule at dir %s", dir)

	testhelpers.AssertTextFilesEqual(t, expectedFile, makefile, "makefile")
}
