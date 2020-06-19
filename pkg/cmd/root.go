package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx-promote/pkg/promote"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	promoteLong = templates.LongDesc(`
		Promotes a version of an application to an Environment
`)

	promoteExample = templates.Examples(`
		# promotes your current app to the staging environment
		%s promote 
	`)
)

// Options the options for this command
type Options struct {
	Options   promote.Options
	BatchMode bool
}

// Main creates a command object for the command
func Main() (*cobra.Command, *promote.Options) {
	options := &promote.Options{}

	cmd := &cobra.Command{
		Use:     "promote",
		Short:   "Promotes a version of an application to an Environment",
		Long:    promoteLong,
		Example: fmt.Sprintf(promoteExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.Environment, opts.OptionEnvironment, "e", "", "The Environment to promote to")
	cmd.Flags().BoolVarP(&options.AllAutomatic, "all-auto", "", false, "Promote to all automatic environments in order")
	cmd.Flags().BoolVarP(&options.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")

	options.AddOptions(cmd)

	return cmd, options
}
