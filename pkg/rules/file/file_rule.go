package file

import (
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// FileRule uses a file rule to create promote pull requests
func FileRule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.FileRule == nil {
		return errors.Errorf("no fileRule configured")
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

	commandLine, err := evaluateTemplate(r, rule.CommandTemplate, rule.LinePrefix)
	if err != nil {
		return errors.Wrapf(err, "failed to create Makefile statement")
	}

	updated := false
	updateTemplate := rule.UpdateTemplate
	if updateTemplate != nil {
		lineMatcher := v1alpha1.LineMatcher{}
		lineMatcher.Prefix, err = evaluateTemplate(r, updateTemplate.Prefix, "")
		if err != nil {
			return errors.Wrapf(err, "failed to evaluate updateTemplate.prefix")
		}
		lineMatcher.Regex, err = evaluateTemplate(r, updateTemplate.Regex, "")
		if err != nil {
			return errors.Wrapf(err, "failed to evaluate updateTemplate.regex")
		}

		m, err := createMatcher(rule, lineMatcher)
		if err != nil {
			return errors.Wrapf(err, "failed to create line matcher for updateTemplate")
		}

		for i, line := range lines {
			if m(line) {
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
				return errors.Wrapf(err, "failed to create line matcher for insertAfter")
			}
			for i, line := range lines {
				if m(line) {
					insertIdx = i
				}
			}
			if insertIdx >= 0 {
				break
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
	regText := lineMatcher.Regex
	if regText != "" {
		r, err := regexp.Compile(regText)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse line match regex: %s", regText)
		}
		return func(line string) bool {
			return r.MatchString(line)
		}, nil
	}
	return nil, errors.Errorf("not supported lime matcher %#v", lineMatcher)
}

func evaluateTemplate(r *rules.PromoteRule, templateText string, linePrefix string) (string, error) {
	if templateText == "" {
		return "", nil
	}
	tmpl, err := template.New("test").Parse(templateText)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse go template: %s", templateText)
	}
	ctx := r.TemplateContext
	buf := &strings.Builder{}
	if linePrefix != "" {
		buf.WriteString(linePrefix)
	}
	err = tmpl.Execute(buf, &ctx)
	if err != nil {
		return buf.String(), errors.Wrapf(err, "failed to evaluate template with %#v", ctx)
	}
	return buf.String(), nil
}
