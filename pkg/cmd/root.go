package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-promote/pkg/common"
	"github.com/jenkins-x/jx-promote/pkg/promote"
	"github.com/spf13/cobra"
)

var (
	promoteLong = templates.LongDesc(`
		Promotes a version of an application to an Environment
`)

	promoteExample = templates.Examples(`
		# promotes your current app to the staging environment
		%s 
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
		Use:     common.BinaryName,
		Short:   "Promotes a version of an application to an Environment",
		Long:    promoteLong,
		Example: fmt.Sprintf(promoteExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.Environment, "env", "e", "", "The Environment to promote to")
	cmd.Flags().StringVarP(&options.DefaultAppNamespace, "default-app-namespace", "", "", "The default namespace for promoting to remote clusters for the first")
	cmd.Flags().StringArrayP("promotion-environments", "", options.PromoteEnvironments, "The environments considered for promotion")
	cmd.Flags().BoolVarP(&options.AllAutomatic, "all-auto", "", false, "Promote to all automatic environments in order")
	cmd.Flags().BoolVarP(&options.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")

	options.AddOptions(cmd)

	return cmd, options
}
