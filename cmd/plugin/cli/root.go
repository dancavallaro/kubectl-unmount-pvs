package cli

import (
	"errors"
	"fmt"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"strings"
)

var (
	config *plugin.ConfigFlags
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kubectl unmount-pvs",
		Short:         "TODO", // TODO
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
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
		StorageClass: stringptr(""),
	}
	// TODO: add dry-run option
	cmd.Flags().StringVarP(config.StorageClass, "storage-class", "c", "", "Unmount PVs of a specific storage class")
	config.AddFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func stringptr(val string) *string {
	return &val
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.AutomaticEnv()
}
