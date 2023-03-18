package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/lmzuccarelli/golang-imageindex-list/pkg/index"
	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/manifest"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/mirror"
	"github.com/spf13/cobra"
)

const (
	logsDir          string = "logs"
	workingDir       string = "working-dir/"
	releaseImageDir  string = "release-images"
	operatorImageDir string = "operator-images"
	ociProtocol      string = "oci://"
	dockerProtocol   string = "docker://"
)

type ExecutorSchema struct {
	Log       clog.PluggableLoggerInterface
	Manifest  manifest.ManifestInterface
	Mirror    mirror.MirrorInterface
	Collector index.CollectorInterface
	Opts      mirror.CopyOptions
}

func NewCliCmd(log clog.PluggableLoggerInterface) *cobra.Command {

	global := &mirror.GlobalOptions{
		TlsVerify:      false,
		InsecurePolicy: true,
	}

	flagSharedOpts, sharedOpts := mirror.SharedImageFlags()
	flagDepTLS, deprecatedTLSVerifyOpt := mirror.DeprecatedTLSVerifyFlags()
	flagSrcOpts, srcOpts := mirror.ImageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	flagDestOpts, destOpts := mirror.ImageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	flagRetryOpts, retryOpts := mirror.RetryFlags()

	opts := mirror.CopyOptions{
		Global:              global,
		DeprecatedTLSVerify: deprecatedTLSVerifyOpt,
		SrcImage:            srcOpts,
		DestImage:           destOpts,
		RetryOpts:           retryOpts,
		Dev:                 false,
	}

	ex := &ExecutorSchema{
		Log:  log,
		Opts: opts,
	}

	cmd := &cobra.Command{
		Use:           "list <image-type select one of [release,operator]> --image-index <image-reference>",
		Version:       "v0.0.1",
		Short:         "List image index by channel and version",
		Long:          "List image index (operators or releases) by channel and version",
		Example:       "list operator --image-index registry.redhat.io/redhat-operator-index:4.11",
		Args:          cobra.MinimumNArgs(1),
		SilenceErrors: false,
		SilenceUsage:  false,
		Run: func(cmd *cobra.Command, args []string) {
			err := ex.Validate(args)
			if err != nil {
				os.Exit(1)
			}

			ex.Complete(args)

			err = ex.Run(cmd, args)
			if err != nil {
				log.Error("%v ", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&opts.Global.LogLevel, "loglevel", "info", "Log level one of (info, debug, trace, error)")
	cmd.Flags().StringVar(&opts.Global.Reference, "image-index", "", "Image index to use")
	cmd.Flags().AddFlagSet(&flagSharedOpts)
	cmd.Flags().AddFlagSet(&flagRetryOpts)
	cmd.Flags().AddFlagSet(&flagDepTLS)
	cmd.Flags().AddFlagSet(&flagSrcOpts)
	cmd.Flags().AddFlagSet(&flagDestOpts)

	return cmd
}

// Run - start the mirror functionality
func (o *ExecutorSchema) Run(cmd *cobra.Command, args []string) error {

	// clean up logs directory
	os.RemoveAll(logsDir)

	// ensure working dir exists
	err := os.MkdirAll(workingDir, 0755)
	if err != nil {
		o.Log.Error(" %v ", err)
		return err
	}

	// create logs directory
	err = os.MkdirAll(logsDir, 0755)
	if err != nil {
		o.Log.Error(" %v ", err)
		return err
	}

	// create release cache dir
	o.Log.Trace("creating release cache directory %s ", o.Opts.Global.Dir+releaseImageDir)
	err = os.MkdirAll(o.Opts.Global.Dir+releaseImageDir, 0755)
	if err != nil {
		o.Log.Error(" %v ", err)
		return err
	}

	// create operator cache dir
	o.Log.Trace("creating operator cache directory %s ", o.Opts.Global.Dir+operatorImageDir)
	err = os.MkdirAll(o.Opts.Global.Dir+operatorImageDir, 0755)
	if err != nil {
		o.Log.Error(" %v ", err)
		return err
	}

	return o.Collector.Collect()
}

// Complete - do the final setup of modules
func (o *ExecutorSchema) Complete(args []string) {
	// override log level
	o.Log.Level(o.Opts.Global.LogLevel)
	o.Opts.Global.Dir = workingDir
	o.Opts.Global.ImageIndex = args[0]

	// initialise all dependant modules
	o.Manifest = manifest.New(o.Log)
	mc := mirror.NewMirrorCopy()
	o.Mirror = mirror.New(mc)
	o.Collector = index.New(o.Log, o.Manifest, o.Mirror, o.Opts)

}

// Validate - cobra validation
func (o *ExecutorSchema) Validate(imgIndex []string) error {
	if o.Opts.Global.Reference == "" {
		return fmt.Errorf("the flag --image-index is required - refer to example usage with --help")
	}
	switch imgIndex[0] {
	case "operator":
		if !strings.Contains(o.Opts.Global.Reference, "operator") {
			return fmt.Errorf("ensure the --image-index is referencing an operator catalog")
		}
	case "release":
		if !strings.Contains(o.Opts.Global.Reference, "release") {
			return fmt.Errorf("ensure the --image-index is referencing a release index")
		}
	default:
		return fmt.Errorf("first argument must be either 'release' or 'operator'")
	}
	o.Log.Info("validate : image-index %v", imgIndex)
	return nil
}
