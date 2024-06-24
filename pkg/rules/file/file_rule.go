package file

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/jenkins-x-plugins/jx-promote/pkg/apis/promote/v1alpha1"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
)

// FileRule uses a file rule to create promote pull requests
func Rule(r *rules.PromoteRule) error {
	config := r.Config
	if config.Spec.FileRule == nil {
		return fmt.Errorf("no fileRule configured")
	}
	rule := config.Spec.FileRule
	path := rule.Path
	if path == "" {
		return fmt.Errorf("no path property in FileRule %#v", rule)
	}
	path = filepath.Join(r.Dir, path)
	exists, err := files.FileExists(path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists %s: %w", path, err)
	}
	if !exists {
		return fmt.Errorf("file does not exist: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")

	commandLine, err := evaluateTemplate(r, rule.CommandTemplate, rule.LinePrefix)
	if err != nil {
		return fmt.Errorf("failed to create Makefile statement: %w", err)
	}

	updated := false
	updateTemplate := rule.UpdateTemplate
	if updateTemplate != nil {
		lineMatcher := v1alpha1.LineMatcher{}
		lineMatcher.Prefix, err = evaluateTemplate(r, updateTemplate.Prefix, "")
		if err != nil {
			return fmt.Errorf("failed to evaluate updateTemplate.prefix: %w", err)
		}
		lineMatcher.Regex, err = evaluateTemplate(r, updateTemplate.Regex, "")
		if err != nil {
			return fmt.Errorf("failed to evaluate updateTemplate.regex: %w", err)
		}

		m, err := createMatcher(rule, lineMatcher)
		if err != nil {
			return fmt.Errorf("failed to create line matcher for updateTemplate: %w", err)
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
				return fmt.Errorf("failed to create line matcher for insertAfter: %w", err)
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
	err = os.WriteFile(path, data, files.DefaultFileWritePermissions)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	log.Logger().Infof("modified file %s", termcolor.ColorInfo(path))
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
			return nil, fmt.Errorf("failed to parse line match regex: %s: %w", regText, err)
		}
		return func(line string) bool {
			return r.MatchString(line)
		}, nil
	}
	return nil, fmt.Errorf("not supported lime matcher %#v", lineMatcher)
}

func evaluateTemplate(r *rules.PromoteRule, templateText, linePrefix string) (string, error) {
	if templateText == "" {
		return "", nil
	}
	tmpl, err := template.New("test").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse go template: %s: %w", templateText, err)
	}
	ctx := r.TemplateContext
	buf := &strings.Builder{}
	if linePrefix != "" {
		buf.WriteString(linePrefix)
	}
	err = tmpl.Execute(buf, &ctx)
	if err != nil {
		return buf.String(), fmt.Errorf("failed to evaluate template with %#v: %w", ctx, err)
	}
	return buf.String(), nil
}
