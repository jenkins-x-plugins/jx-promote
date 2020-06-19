package rules

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

var (
	makeLinePrefix = "\t"
)

// MakefileRule applies the Makefile rule to the file
func MakefileRule(r *PromoteRule) error {
	config := r.Config
	if config.Spec.MakefileRule == nil {
		return errors.Errorf("no makefile rule configured")
	}
	rule := config.Spec.MakefileRule
	path := rule.Path
	if path == "" {
		path = "Makefile"
	}
	path = filepath.Join(r.Dir, path)
	exists, err := util.FileExists(path)
	if err != nil {
		return errors.Wrapf(err, "failed to check if file exists %s", path)
	}
	if !exists {
		return errors.Errorf("file does not exist: %s", path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", path)
	}

	lines := strings.Split(string(data), "\n")

	line, err := evaluateTemplate(r, rule.CommandTemplate)
	if err != nil {
		return errors.Wrapf(err, "failed to create Makefile statement")
	}
	line = makeLinePrefix + line
	updatePrefix, err := evaluateTemplate(r, rule.UpdatePrefixTemplate)
	if err != nil {
		return errors.Wrapf(err, "failed to create update prefix")
	}
	insertAfterPrefix := rule.InsertAfterPrefix

	updated := false
	if updatePrefix != "" {
		updatePrefix = makeLinePrefix + updatePrefix
	}
	if insertAfterPrefix != "" {
		insertAfterPrefix = makeLinePrefix + insertAfterPrefix
	}
	insertIdx := -1
	for i, line := range lines {
		if updatePrefix != "" && strings.HasPrefix(line, updatePrefix) {
			updated = true
			lines[i] = line
			break
		}
		if insertAfterPrefix != "" && strings.HasPrefix(line, insertAfterPrefix) {
			insertIdx = i
		}
	}
	if insertIdx >= 0 {
		updated = true
		lines = insertItem(lines, insertIdx+1, line)
	}
	if !updated {
		lines = append(lines, line)
	}

	data = []byte(strings.Join(lines, "\n"))
	err = ioutil.WriteFile(path, data, util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", path)
	}
	log.Logger().Infof("modified file %s", util.ColorInfo(path))
	return nil
}

func insertItem(a []string, index int, value string) []string {
	if index >= len(a) {
		return append(a, value)
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a
}

func evaluateTemplate(r *PromoteRule, templateText string) (string, error) {
	tmpl, err := template.New("test").Parse(templateText)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse go template: %s", templateText)
	}
	ctx := TemplateContext{
		GitURL:  r.GitURL,
		Version: r.Version,
		AppName: r.AppName,
	}
	buf := &strings.Builder{}
	err = tmpl.Execute(buf, &ctx)
	if err != nil {
		return buf.String(), errors.Wrapf(err, "failed to evaluate template with %#v", ctx)
	}
	return buf.String(), nil
}
