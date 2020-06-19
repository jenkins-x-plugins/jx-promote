package cmd

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx-promote/pkg/promote"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
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
	Options   promote.PromoteOptions
	BatchMode bool
}

// Main creates a command object for the command
func Main() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "promote",
		Short:   "Promotes a version of an application to an Environment",
		Long:    promoteLong,
		Example: fmt.Sprintf(promoteExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	options := &o.Options
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.Environment, opts.OptionEnvironment, "e", "", "The Environment to promote to")
	cmd.Flags().BoolVarP(&options.AllAutomatic, "all-auto", "", false, "Promote to all automatic environments in order")
	cmd.Flags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")

	options.AddPromoteOptions(cmd)
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	po := &o.Options
	if po.CommonOptions == nil {
		f := clients.NewFactory()
		po.CommonOptions = opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
		po.CommonOptions.BatchMode = o.BatchMode
	}
	return po.Run()
}
