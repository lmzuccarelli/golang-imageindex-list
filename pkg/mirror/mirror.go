package mirror

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

const (
	mirrorToDisk = "mirrorToDisk"
	diskToMirror = "diskToMirror"
)

// MirrorInterface  used to mirror images with container/images (skopeo)
type MirrorInterface interface {
	Run(ctx context.Context, src, dest, mode string, opts *CopyOptions, stdout bufio.Writer) error
}

type MirrorCopyInterface interface {
	CopyImage(ctx context.Context, pc *signature.PolicyContext, destRef, srcRef types.ImageReference, opts *copy.Options) ([]byte, error)
}

// Mirror
type Mirror struct {
	mc   MirrorCopyInterface
	Mode string
}

type MirrorCopy struct{}

// New returns new Mirror instance
func New(mc MirrorCopyInterface) MirrorInterface {
	return &Mirror{mc: mc}
}

func NewMirrorCopy() MirrorCopyInterface {
	return &MirrorCopy{}
}

// Run - method to copy images from source to destination
func (o *Mirror) Run(ctx context.Context, src, dest, mode string, opts *CopyOptions, stdout bufio.Writer) (err error) {
	return o.copy(ctx, src, dest, opts, stdout)
}

func (o *MirrorCopy) CopyImage(ctx context.Context, pc *signature.PolicyContext, destRef, srcRef types.ImageReference, co *copy.Options) ([]byte, error) {
	return copy.Image(ctx, pc, destRef, srcRef, co)
}

// copy - copy images setup and execute
func (o *Mirror) copy(ctx context.Context, src, dest string, opts *CopyOptions, out bufio.Writer) (retErr error) {
	opts.DeprecatedTLSVerify.WarnIfUsed([]string{"--src-tls-verify", "--dest-tls-verify"})
	opts.RemoveSignatures, _ = strconv.ParseBool("true")
	if err := ReexecIfNecessaryForImages([]string{src, dest}...); err != nil {
		return err
	}
	policyContext, err := opts.Global.GetPolicyContext()
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer func() {
		if err := policyContext.Destroy(); err != nil {
			retErr = fmt.Errorf("%v", err)
		}
	}()

	srcRef, err := alltransports.ParseImageName(src)
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", src, err)
	}
	destRef, err := alltransports.ParseImageName(dest)
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", dest, err)
	}

	sourceCtx, err := opts.SrcImage.NewSystemContext()
	if err != nil {
		return err
	}
	destinationCtx, err := opts.DestImage.NewSystemContext()
	if err != nil {
		return err
	}

	var manifestType string
	if len(opts.Format) > 0 {
		manifestType, err = ParseManifestFormat(opts.Format)
		if err != nil {
			return err
		}
	}

	ctx, cancel := opts.Global.CommandTimeoutContext()
	defer cancel()

	//opts.DigestFile = "test-digest"
	writer := io.Writer(&out)

	co := &copy.Options{
		RemoveSignatures:             opts.RemoveSignatures,
		SignBy:                       opts.SignByFingerprint,
		SignBySigstorePrivateKeyFile: opts.SignBySigstorePrivateKey,
		ReportWriter:                 writer,
		SourceCtx:                    sourceCtx,
		DestinationCtx:               destinationCtx,
		ForceManifestMIMEType:        manifestType,
		PreserveDigests:              opts.PreserveDigests,
	}

	return retry.IfNecessary(ctx, func() error {
		//manifestBytes, err := copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		_, err := o.mc.CopyImage(ctx, policyContext, destRef, srcRef, co)
		if err != nil {
			return err
		}
		out.Flush()
		return nil
	}, opts.RetryOpts)
}
