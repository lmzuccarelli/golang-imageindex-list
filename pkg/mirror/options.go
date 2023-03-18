package mirror

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/google/uuid"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const defaultUserAgent string = "skopeo/v.19.5"

// errorShouldDisplayUsage is a subtype of error used by command handlers to indicate that cli.ShowSubcommandHelp should be called.
type ErrorShouldDisplayUsage struct {
	Error error
}

// deprecatedTLSVerifyOption represents a deprecated --tls-verify option,
// which was accepted for all subcommands, for a time.
// Every user should call deprecatedTLSVerifyOption.warnIfUsed() as part of handling the CLI,
// whether or not the value actually ends up being used.
// DO NOT ADD ANY NEW USES OF THIS; just call dockerImageFlags with an appropriate, possibly empty, flagPrefix.
type DeprecatedTLSVerifyOption struct {
	tlsVerify commonFlag.OptionalBool // FIXME FIXME: Warn if this is used, or even if it is ignored.
}

// warnIfUsed warns if tlsVerify was set by the user, and suggests alternatives (which should
// start with "--").
// Every user should call this as part of handling the CLI, whether or not the value actually
// ends up being used.
func (opts *DeprecatedTLSVerifyOption) WarnIfUsed(alternatives []string) {
	if opts.tlsVerify.Present() {
		logrus.Warnf("'--tls-verify' is deprecated, instead use: %s", strings.Join(alternatives, ", "))
	}
}

// deprecatedTLSVerifyFlags prepares the CLI flag writing into deprecatedTLSVerifyOption, and the managed deprecatedTLSVerifyOption structure.
// DO NOT ADD ANY NEW USES OF THIS; just call dockerImageFlags with an appropriate, possibly empty, flagPrefix.
func DeprecatedTLSVerifyFlags() (pflag.FlagSet, *DeprecatedTLSVerifyOption) {
	opts := DeprecatedTLSVerifyOption{}
	fs := pflag.FlagSet{}
	flag := commonFlag.OptionalBoolFlag(&fs, &opts.tlsVerify, "tls-verify", "require HTTPS and verify certificates when accessing the container registry")
	flag.Hidden = true
	return fs, &opts
}

// sharedImageOptions collects CLI flags which are image-related, but do not change across images.
// This really should be a part of globalOptions, but that would break existing users of (skopeo copy --authfile=).
type SharedImageOptions struct {
	authFilePath string // Path to a */containers/auth.json
}

// sharedImageFlags prepares a collection of CLI flags writing into sharedImageOptions, and the managed sharedImageOptions structure.
func SharedImageFlags() (pflag.FlagSet, *SharedImageOptions) {
	opts := SharedImageOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.authFilePath, "authfile", os.Getenv("REGISTRY_AUTH_FILE"), "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json")
	return fs, &opts
}

// dockerImageOptions collects CLI flags specific to the "docker" transport, which are
// the same across subcommands, but may be different for each image
// (e.g. may differ between the source and destination of a copy)
type dockerImageOptions struct {
	global              *GlobalOptions             // May be shared across several imageOptions instances.
	shared              *SharedImageOptions        // May be shared across several imageOptions instances.
	deprecatedTLSVerify *DeprecatedTLSVerifyOption // May be shared across several imageOptions instances, or nil.
	authFilePath        commonFlag.OptionalString  // Path to a */containers/auth.json (prefixed version to override shared image option).
	credsOption         commonFlag.OptionalString  // username[:password] for accessing a registry
	userName            commonFlag.OptionalString  // username for accessing a registry
	password            commonFlag.OptionalString  // password for accessing a registry
	registryToken       commonFlag.OptionalString  // token to be used directly as a Bearer token when accessing the registry
	dockerCertPath      string                     // A directory using Docker-like *.{crt,cert,key} files for connecting to a registry or a daemon
	tlsVerify           commonFlag.OptionalBool    // Require HTTPS and verify certificates (for docker: and docker-daemon:)
	noCreds             bool                       // Access the registry anonymously
}

// imageOptions collects CLI flags which are the same across subcommands, but may be different for each image
// (e.g. may differ between the source and destination of a copy)
type imageOptions struct {
	dockerImageOptions
	sharedBlobDir    string // A directory to use for OCI blobs, shared across repositories
	dockerDaemonHost string // docker-daemon: host to connect to
}

// dockerImageFlags prepares a collection of docker-transport specific CLI flags
// writing into imageOptions, and the managed imageOptions structure.
func dockerImageFlags(global *GlobalOptions, shared *SharedImageOptions, deprecatedTLSVerify *DeprecatedTLSVerifyOption, flagPrefix, credsOptionAlias string) (pflag.FlagSet, *imageOptions) {
	flags := imageOptions{
		dockerImageOptions: dockerImageOptions{
			global:              global,
			shared:              shared,
			deprecatedTLSVerify: deprecatedTLSVerify,
		},
	}

	fs := pflag.FlagSet{}
	if flagPrefix != "" {
		// the non-prefixed flag is handled by a shared flag.
		fs.Var(commonFlag.NewOptionalStringValue(&flags.authFilePath), flagPrefix+"authfile", "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json")
	}
	fs.Var(commonFlag.NewOptionalStringValue(&flags.credsOption), flagPrefix+"creds", "Use `USERNAME[:PASSWORD]` for accessing the registry")
	fs.Var(commonFlag.NewOptionalStringValue(&flags.userName), flagPrefix+"username", "Username for accessing the registry")
	fs.Var(commonFlag.NewOptionalStringValue(&flags.password), flagPrefix+"password", "Password for accessing the registry")
	if credsOptionAlias != "" {
		// This is horribly ugly, but we need to support the old option forms of (skopeo copy) for compatibility.
		// Don't add any more cases like this.
		f := fs.VarPF(commonFlag.NewOptionalStringValue(&flags.credsOption), credsOptionAlias, "", "Use `USERNAME[:PASSWORD]` for accessing the registry")
		f.Hidden = true
	}
	fs.Var(commonFlag.NewOptionalStringValue(&flags.registryToken), flagPrefix+"registry-token", "Provide a Bearer token for accessing the registry")
	fs.StringVar(&flags.dockerCertPath, flagPrefix+"cert-dir", "", "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the registry or daemon")
	commonFlag.OptionalBoolFlag(&fs, &flags.tlsVerify, flagPrefix+"tls-verify", "require HTTPS and verify certificates when talking to the container registry or daemon")
	fs.BoolVar(&flags.noCreds, flagPrefix+"no-creds", false, "Access the registry anonymously")
	return fs, &flags
}

// imageFlags prepares a collection of CLI flags writing into imageOptions, and the managed imageOptions structure.
func ImageFlags(global *GlobalOptions, shared *SharedImageOptions, deprecatedTLSVerify *DeprecatedTLSVerifyOption, flagPrefix, credsOptionAlias string) (pflag.FlagSet, *imageOptions) {
	dockerFlags, opts := dockerImageFlags(global, shared, deprecatedTLSVerify, flagPrefix, credsOptionAlias)

	fs := pflag.FlagSet{}
	fs.StringVar(&opts.sharedBlobDir, flagPrefix+"shared-blob-dir", "", "`DIRECTORY` to use to share blobs across OCI repositories")
	fs.StringVar(&opts.dockerDaemonHost, flagPrefix+"daemon-host", "", "use docker daemon host at `HOST` (docker-daemon: only)")
	fs.AddFlagSet(&dockerFlags)
	return fs, opts
}

func RetryFlags() (pflag.FlagSet, *retry.Options) {
	opts := retry.Options{}
	fs := pflag.FlagSet{}
	fs.IntVar(&opts.MaxRetry, "retry-times", 0, "the number of times to possibly retry")
	return fs, &opts
}

// getPolicyContext returns a *signature.PolicyContext based on opts.
func (opts *GlobalOptions) GetPolicyContext() (*signature.PolicyContext, error) {
	var policy *signature.Policy // This could be cached across calls in opts.
	var err error
	if opts.InsecurePolicy {
		policy = &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	} else if opts.PolicyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(opts.PolicyPath)
	}
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

// commandTimeoutContext returns a context.Context and a cancellation callback based on opts.
// The caller should usually "defer cancel()" immediately after calling this.
func (opts *GlobalOptions) CommandTimeoutContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	var cancel context.CancelFunc = func() {
		// empty function - its ok for now
	}
	if opts.CommandTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.CommandTimeout)
	}
	return ctx, cancel
}

// newSystemContext returns a *types.SystemContext corresponding to opts.
// It is guaranteed to return a fresh instance, so it is safe to make additional updates to it.
func (opts *GlobalOptions) NewSystemContext() *types.SystemContext {
	ctx := &types.SystemContext{
		RegistriesDirPath:        opts.RegistriesDirPath,
		ArchitectureChoice:       opts.OverrideArch,
		OSChoice:                 opts.OverrideOS,
		VariantChoice:            opts.OverrideVariant,
		SystemRegistriesConfPath: opts.RegistriesConfPath,
		BigFilesTemporaryDir:     opts.TmpDir,
		DockerRegistryUserAgent:  defaultUserAgent,
	}
	// DEPRECATED: We support this for backward compatibility, but override it if a per-image flag is provided.
	if !opts.TlsVerify {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}
	return ctx
}

// newSystemContext returns a *types.SystemContext corresponding to opts.
// It is guaranteed to return a fresh instance, so it is safe to make additional updates to it.
func (opts *imageOptions) NewSystemContext() (*types.SystemContext, error) {
	// *types.SystemContext instance from globalOptions
	//  imageOptions option overrides the instance if both are present.
	ctx := opts.global.NewSystemContext()
	ctx.DockerCertPath = opts.dockerCertPath
	ctx.OCISharedBlobDirPath = opts.sharedBlobDir
	ctx.AuthFilePath = opts.shared.authFilePath
	ctx.DockerDaemonHost = opts.dockerDaemonHost
	ctx.DockerDaemonCertPath = opts.dockerCertPath

	return ctx, nil
}

// imageDestOptions is a superset of imageOptions specialized for image destinations.
type imageDestOptions struct {
	*imageOptions
	dirForceCompression         bool                   // Compress layers when saving to the dir: transport
	dirForceDecompression       bool                   // Decompress layers when saving to the dir: transport
	ociAcceptUncompressedLayers bool                   // Whether to accept uncompressed layers in the oci: transport
	compressionFormat           string                 // Format to use for the compression
	compressionLevel            commonFlag.OptionalInt // Level to use for the compression
	precomputeDigests           bool                   // Precompute digests to dedup layers when saving to the docker: transport
}

// imageDestFlags prepares a collection of CLI flags writing into imageDestOptions, and the managed imageDestOptions structure.
func ImageDestFlags(global *GlobalOptions, shared *SharedImageOptions, deprecatedTLSVerify *DeprecatedTLSVerifyOption, flagPrefix, credsOptionAlias string) (pflag.FlagSet, *imageDestOptions) {
	genericFlags, genericOptions := ImageFlags(global, shared, deprecatedTLSVerify, flagPrefix, credsOptionAlias)
	opts := imageDestOptions{imageOptions: genericOptions}
	fs := pflag.FlagSet{}
	fs.AddFlagSet(&genericFlags)
	fs.BoolVar(&opts.dirForceCompression, flagPrefix+"compress", false, "Compress tarball image layers when saving to directory using the 'dir' transport. (default is same compression type as source)")
	fs.BoolVar(&opts.dirForceDecompression, flagPrefix+"decompress", false, "Decompress tarball image layers when saving to directory using the 'dir' transport. (default is same compression type as source)")
	fs.BoolVar(&opts.ociAcceptUncompressedLayers, flagPrefix+"oci-accept-uncompressed-layers", false, "Allow uncompressed image layers when saving to an OCI image using the 'oci' transport. (default is to compress things that aren't compressed)")
	fs.StringVar(&opts.compressionFormat, flagPrefix+"compress-format", "", "`FORMAT` to use for the compression")
	fs.Var(commonFlag.NewOptionalIntValue(&opts.compressionLevel), flagPrefix+"compress-level", "`LEVEL` to use for the compression")
	fs.BoolVar(&opts.precomputeDigests, flagPrefix+"precompute-digests", false, "Precompute digests to prevent uploading layers already on the registry using the 'docker' transport.")
	return fs, &opts
}

// parseManifestFormat parses format parameter for copy and sync command.
// It returns string value to use as manifest MIME type
func ParseManifestFormat(manifestFormat string) (string, error) {
	switch manifestFormat {
	case "oci":
		return imgspecv1.MediaTypeImageManifest, nil
	case "v2s1":
		return manifest.DockerV2Schema1SignedMediaType, nil
	case "v2s2":
		return manifest.DockerV2Schema2MediaType, nil
	default:
		return "", fmt.Errorf("unknown format %q. Choose one of the supported formats: 'oci', 'v2s1', or 'v2s2'", manifestFormat)
	}
}

type GlobalOptions struct {
	LogLevel           string        // one of info, debug, trace
	TlsVerify          bool          // Require HTTPS and verify certificates (for docker: and docker-daemon:)
	PolicyPath         string        // Path to a signature verification policy file
	InsecurePolicy     bool          // Use an "allow everything" signature verification policy
	RegistriesDirPath  string        // Path to a "registries.d" registry configuration directory
	OverrideArch       string        // Architecture to use for choosing images, instead of the runtime one
	OverrideOS         string        // OS to use for choosing images, instead of the runtime one
	OverrideVariant    string        // Architecture variant to use for choosing images, instead of the runtime one
	CommandTimeout     time.Duration // Timeout for the command execution
	RegistriesConfPath string        // Path to the "registries.conf" file
	TmpDir             string        // Path to use for big temporary files
	Dir                string        // working directory
	ConfigPath         string        // Path to use for imagesetconfig
	From               string        // Used for mirroring (diskToMirror)
	Quiet              bool          // Suppress output information when copying images
	Force              bool          // Force the copy/mirror even if there is nothing to update
	ImageIndex         string        // operator , release
	Reference          string        //
}

type CopyOptions struct {
	Global                   *GlobalOptions
	DeprecatedTLSVerify      *DeprecatedTLSVerifyOption
	SrcImage                 *imageOptions
	DestImage                *imageDestOptions
	RetryOpts                *retry.Options
	AdditionalTags           []string  // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these
	RemoveSignatures         bool      // Do not copy signatures from the source image
	SignByFingerprint        string    // Sign the image using a GPG key with the specified fingerprint
	SignBySigstorePrivateKey string    // Sign the image using a sigstore private key
	SignPassphraseFile       string    // Path pointing to a passphrase file when signing (for either signature format, but only one of them)
	SignIdentity             string    // Identity of the signed image, must be a fully specified docker reference
	DigestFile               string    // Write digest to this file
	Format                   string    // Force conversion of the image to a specified format
	All                      bool      // Copy all of the images if the source is a list
	MultiArch                string    // How to handle multi architecture images
	PreserveDigests          bool      // Preserve digests during copy
	EncryptLayer             []int     // The list of layers to encrypt
	EncryptionKeys           []string  // Keys needed to encrypt the image
	DecryptionKeys           []string  // Keys needed to decrypt the image
	Mode                     string    // 2 options disktoMirror or mirrorToDisk (for now)
	Dev                      bool      // developer mode - will be removed when completed
	Destination              string    // what to target to
	UUID                     uuid.UUID // set uuid
	ImageType                string    // release, catalog-operator, additionalImage
}
