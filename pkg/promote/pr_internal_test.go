//go:build unit
// +build unit

package promote

import (
	"testing"

	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSparsePatternForRule(t *testing.T) {
	testCases := []struct {
		name     string
		spec     v1alpha1.PromoteSpec
		appName  string
		expected string
	}{
		{
			name:     "file rule modifies a single file",
			spec:     v1alpha1.PromoteSpec{FileRule: &v1alpha1.FileRule{Path: "Makefile"}},
			appName:  "myapp",
			expected: "/Makefile",
		},
		{
			name:     "file rule with a nested path",
			spec:     v1alpha1.PromoteSpec{FileRule: &v1alpha1.FileRule{Path: "config/values.yaml"}},
			appName:  "myapp",
			expected: "/config/values.yaml",
		},
		{
			name:     "file rule with a leading slash is normalised",
			spec:     v1alpha1.PromoteSpec{FileRule: &v1alpha1.FileRule{Path: "/Makefile"}},
			appName:  "myapp",
			expected: "/Makefile",
		},
		{
			name:     "file rule without a path cannot be derived",
			spec:     v1alpha1.PromoteSpec{FileRule: &v1alpha1.FileRule{}},
			appName:  "myapp",
			expected: "",
		},
		{
			name:     "kpt rule covers the namespace/app subtree",
			spec:     v1alpha1.PromoteSpec{KptRule: &v1alpha1.KptRule{Path: "namespaces/jx-staging"}},
			appName:  "myapp",
			expected: "/namespaces/jx-staging/myapp/",
		},
		{
			name:     "kpt rule with an empty path falls back to the app dir at root",
			spec:     v1alpha1.PromoteSpec{KptRule: &v1alpha1.KptRule{}},
			appName:  "myapp",
			expected: "/myapp/",
		},
		{
			name:     "kpt rule with neither path nor app cannot be derived",
			spec:     v1alpha1.PromoteSpec{KptRule: &v1alpha1.KptRule{}},
			appName:  "",
			expected: "",
		},
		{
			name:     "helm/helmfile rules need no extra pattern",
			spec:     v1alpha1.PromoteSpec{HelmfileRule: &v1alpha1.HelmfileRule{Path: "helmfile.yaml"}},
			appName:  "myapp",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sparsePatternForRule(tc.spec, tc.appName), tc.name)
		})
	}
}

func TestIsSparseCheckout(t *testing.T) {
	dir := t.TempDir()
	gitter := cli.NewCLIClient("", cmdrunner.QuietCommandRunner)

	_, err := gitter.Command(dir, "init")
	require.NoError(t, err, "git init")

	// a plain clone/init is not in sparse-checkout mode (mirrors Create()'s full-clone fallback)
	assert.False(t, isSparseCheckout(gitter, dir), "fresh repo should not be sparse")

	// enabling sparse checkout is exactly what SparseCloneToDir does via `sparse-checkout set`
	_, err = gitter.Command(dir, "sparse-checkout", "set", "--no-cone", "/x")
	require.NoError(t, err, "git sparse-checkout set")
	assert.True(t, isSparseCheckout(gitter, dir), "repo should be sparse after sparse-checkout set")
}
