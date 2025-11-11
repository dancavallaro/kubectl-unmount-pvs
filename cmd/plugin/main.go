package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/common"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	config *plugin.ConfigFlags
)

func main() {
	if err := RootCmd().Execute(); err != nil {
		_, _ = fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kubectl unmount-pvs",
		Short:         "Unmount all PersistentVolumes of a particular StorageClass",
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if *config.Namespace == "" && *config.StorageClass == "" {
				return fmt.Errorf("you must specify at least one of --namespace or --storage-class")
			}
			if err := plugin.RunPlugin(config); err != nil {
				return errors.Unwrap(err)
			}
			return nil
		},
	}

	cobra.OnInitialize(initConfig)
	config = &plugin.ConfigFlags{
		ConfigFlags:  *genericclioptions.NewConfigFlags(false),
		Confirmed:    common.BoolP(false),
		DryRun:       common.BoolP(false),
		StorageClass: common.StringP(""),
	}

	cmd.Flags().StringVarP(config.StorageClass, "storage-class", "c", "", "Unmount PVs of a specific storage class")
	cmd.Flags().BoolVarP(config.DryRun, "dry-run", "d", false,
		"Print summary of controllers that would be scaled down, but *don't* modify anything")
	cmd.Flags().BoolVarP(config.Confirmed, "yes", "y", false, "Skip confirmation prompt and proceed with scaling down pods")
	config.AddFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func initConfig() {
	viper.AutomaticEnv()
}
