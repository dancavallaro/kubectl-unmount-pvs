package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dancavallaro/kubectl-unmount/pkg/common"
	"github.com/dancavallaro/kubectl-unmount/pkg/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	// Import cloud auth providers
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	config *plugin.ConfigFlags

	// Injected by goreleaser via ldflags at build-time
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := RootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kubectl unmount",
		Short:         "Unmount all PersistentVolumes of a particular StorageClass",
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if *config.Namespace == "" && *config.StorageClass == "" {
				return errors.New("you must specify at least one of --namespace or --storage-class")
			}
			if *config.StorageClass != "" && *config.PVCName != "" {
				return errors.New("cannot specify both --storage-class and --pvc-name")
			}
			if err := plugin.RunPlugin(config); err != nil {
				return errors.Unwrap(err)
			}
			return nil
		},
		Version: fmt.Sprintf("kubectl-unmount v%s, commit %s, built at %s", version, commit, date),
	}
	cmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the current version of kubectl-unmount",
		Run: func(cmd *cobra.Command, args []string) {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			_ = root.Execute()
		},
	}
	cmd.AddCommand(versionCmd)

	cobra.OnInitialize(initConfig)
	config = &plugin.ConfigFlags{
		ConfigFlags:  *genericclioptions.NewConfigFlags(false),
		Confirmed:    common.BoolP(false),
		DryRun:       common.BoolP(false),
		PVCName:      common.StringP(""),
		StorageClass: common.StringP(""),
	}

	cmd.Flags().StringVar(config.PVCName, "pvc", "", "Unmount a specific PVC")
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
