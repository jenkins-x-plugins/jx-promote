package rules

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/apis/boot/v1alpha1"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// FileRule uses a file rule to create promote pull requests
func FileRule(r *PromoteRule) error {
	config := r.Config
	if config.Spec.FileRule == nil {
		return errors.Errorf("no makefile rule configured")
	}
	rule := config.Spec.FileRule
	path := rule.Path
	if path == "" {
		return errors.Errorf("no path property in FileRule %#v", rule)
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

	commandLine, err := evaluateTemplate(r, rule.CommandTemplate)
	if err != nil {
		return errors.Wrapf(err, "failed to create Makefile statement")
	}
	updatePrefix, err := evaluateTemplate(r, rule.UpdatePrefixTemplate)
	if err != nil {
		return errors.Wrapf(err, "failed to create update prefix")
	}

	linePrefix := rule.LinePrefix
	if linePrefix != "" {
		commandLine = linePrefix + commandLine
		if updatePrefix != "" {
			updatePrefix = linePrefix + updatePrefix
		}
	}

	updated := false
	if updatePrefix != "" {
		for i, line := range lines {
			if strings.HasPrefix(line, updatePrefix) {
				updated = true
				lines[i] = commandLine
				break
			}
		}
	}
	if !updated {
		insertIdx := -1
		for _, insertAfter := range rule.InsertAfter {
			m, err := createMatcher(rule, insertAfter)
			if err != nil {
				return errors.Wrapf(err, "failed to create matcher")
			}

			for i, line := range lines {
				if m(line) {
					insertIdx = i
				}
			}
		}
		if insertIdx >= 0 {
			updated = true
			lines = insertItem(lines, insertIdx+1, commandLine)
		}
		if !updated {
			lines = append(lines, commandLine)
		}
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

func createMatcher(rule *v1alpha1.FileRule, lineMatcher v1alpha1.LineMatcher) (func(string) bool, error) {
	linePrefix := rule.LinePrefix

	prefix := lineMatcher.Prefix
	if prefix != "" {
		prefix2 := linePrefix + prefix
		return func(line string) bool {
			if strings.HasPrefix(line, prefix) {
				return true
			}
			if linePrefix != "" {
				return strings.HasPrefix(line, prefix2)
			}
			return false
		}, nil
	}
	return nil, errors.Errorf("not supported lime matcher %#v", lineMatcher)
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
