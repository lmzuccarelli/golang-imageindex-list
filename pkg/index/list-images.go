package index

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/manifest"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/mirror"
)

const (
	operatorImageDir        string = "operator-images"
	releaseImageDir         string = "release-images"
	operatorImageExtractDir string = "hold-operators"
	releaseImageExtractDir  string = "hold-releases"
	blobsDir                string = "blobs/sha256"
	errMsg                  string = "[Collect] %v "
	dockerProtocol          string = "docker://"
	ociProtocol             string = "oci://"
	ociProtocolTrimmed      string = "oci:"
	logsFile                string = "logs/util.log"
	releaseJSONPath         string = "release-manifests/image-references"
	releaseManifest         string = "release-manifests"
)

type CollectorInterface interface {
	Collect() error
}

func New(log clog.PluggableLoggerInterface,
	manifest manifest.ManifestInterface,
	mirror mirror.MirrorInterface,
	opts mirror.CopyOptions,
) CollectorInterface {
	return &Collector{Log: log, Manifest: manifest, Mirror: mirror, Opts: opts}
}

type Collector struct {
	Log      clog.PluggableLoggerInterface
	Manifest manifest.ManifestInterface
	Mirror   mirror.MirrorInterface
	Opts     mirror.CopyOptions
}

func (o *Collector) Collect() error {

	var label = "/configs"
	f, err := os.Create(logsFile)
	if err != nil {
		o.Log.Error(errMsg, err)
	}
	writer := bufio.NewWriter(f)

	// first check if the artifact of interest has been downloaded
	// if so then skip this step
	hld := strings.Split(o.Opts.Global.Reference, "/")
	imageIndexDir := strings.Replace(hld[len(hld)-1], ":", "/", -1)

	if strings.Contains(o.Opts.Global.ImageIndex, "operator") {
		// setup directory
		cacheDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageExtractDir, imageIndexDir}, "/")
		if _, err := os.Stat(cacheDir); errors.Is(err, os.ErrNotExist) {
			copyDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageDir, imageIndexDir}, "/")
			err := os.MkdirAll(copyDir, 0755)
			if err != nil {
				return err
			}
			o.Log.Info("copying %s image ", o.Opts.Global.Reference)
			src := dockerProtocol + o.Opts.Global.Reference
			dest := ociProtocolTrimmed + copyDir
			err = o.Mirror.Run(context.TODO(), src, dest, "copy", &o.Opts, *writer)
			writer.Flush()
			if err != nil {
				//o.Log.Error(errMsg, err)
				return err
			}

			// it's in oci format so we can go directly to the index.json file
			oci, err := o.Manifest.GetImageIndex(copyDir)
			if err != nil {
				return err
			}

			//read the link to the manifest
			if len(oci.Manifests) == 0 {
				return fmt.Errorf("[Inspect] no manifests found for %s ", o.Opts.Global.Reference)
			} else {
				if !strings.Contains(oci.Manifests[0].Digest, "sha256") {
					return fmt.Errorf("[Inspect] the digest seems to incorrect for %s ", o.Opts.Global.Reference)
				}
			}
			manifest := strings.Split(oci.Manifests[0].Digest, ":")[1]
			o.Log.Info("manifest %v", manifest)

			// read the operator image manifest
			manifestDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageDir, imageIndexDir, blobsDir, manifest}, "/")
			oci, err = o.Manifest.GetImageManifest(manifestDir)
			if err != nil {
				return err
			}

			// read the config digest to get the detailed manifest
			// looking for the label to search for a specific folder
			catalogDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageDir, imageIndexDir, blobsDir, strings.Split(oci.Config.Digest, ":")[1]}, "/")
			ocs, err := o.Manifest.GetOperatorConfig(catalogDir)
			if err != nil {
				return err
			}

			label := ocs.Config.Labels.OperatorsOperatorframeworkIoIndexConfigsV1
			o.Log.Info("label %s", label)

			// untar all the blobs for the operator
			// if the layer with "label (from previous step) is found to a specific folder"
			fromDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageDir, imageIndexDir, blobsDir}, "/")
			toDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageExtractDir, imageIndexDir}, "/")
			err = o.Manifest.ExtractLayersOCI(fromDir, toDir, label, oci)
			if err != nil {
				return err
			}
		}

		configsDir := strings.Join([]string{o.Opts.Global.Dir, operatorImageExtractDir, imageIndexDir, label}, "/")
		files, err := os.ReadDir(configsDir)
		if err != nil {
			return err
		}

		o.Log.Debug("List channels for %s", o.Opts.Global.Reference)
		o.Log.Debug("List of all operators in %s", configsDir)
		for _, file := range files {
			o.Log.Info("\x1b[1;95m %s \x1b[0m", file.Name())
			err := o.Manifest.ListCatalogs(configsDir, file.Name())
			if err != nil {
				o.Log.Error("%v", err)
			}
		}
	}

	if strings.Contains(o.Opts.Global.ImageIndex, "release") {
		// setup cache directory
		cacheDir := strings.Join([]string{o.Opts.Global.Dir, releaseImageExtractDir, imageIndexDir}, "/")
		if _, err := os.Stat(cacheDir); errors.Is(err, os.ErrNotExist) {
			copyDir := strings.Join([]string{o.Opts.Global.Dir, releaseImageDir, imageIndexDir}, "/")
			err := os.MkdirAll(copyDir, 0755)
			if err != nil {
				return err
			}
			o.Log.Info("copying %s image ", o.Opts.Global.Reference)
			src := dockerProtocol + o.Opts.Global.Reference
			dest := ociProtocolTrimmed + copyDir
			err = o.Mirror.Run(context.TODO(), src, dest, "copy", &o.Opts, *writer)
			writer.Flush()
			if err != nil {
				return err
			}

			oci, err := o.Manifest.GetImageIndex(copyDir)
			if err != nil {
				o.Log.Error("[Collector] %v ", err)
				return fmt.Errorf(errMsg, err)
			}

			//read the link to the manifest
			if len(oci.Manifests) == 0 {
				return fmt.Errorf(errMsg, "image index not found ")
			}
			manifest := strings.Split(oci.Manifests[0].Digest, ":")[1]
			o.Log.Debug("image index %v", manifest)

			manifestDir := strings.Join([]string{copyDir, blobsDir, manifest}, "/")
			oci, err = o.Manifest.GetImageManifest(manifestDir)
			if err != nil {
				return fmt.Errorf(errMsg, err)
			}
			o.Log.Debug("manifest %v ", oci.Config.Digest)
			// untar all the blobs for the operator
			// if the layer with "label (from previous step) is found to a specific folder"
			fromDir := strings.Join([]string{o.Opts.Global.Dir, releaseImageDir, imageIndexDir, blobsDir}, "/")
			toDir := strings.Join([]string{o.Opts.Global.Dir + releaseImageExtractDir + imageIndexDir}, "/")
			err = o.Manifest.ExtractLayersOCI(fromDir, toDir, releaseManifest, oci)
			if err != nil {
				return err
			}
		}

		o.Log.Debug("List release for %s", o.Opts.Global.Reference)
		artifactsDir := strings.Join([]string{o.Opts.Global.Dir, releaseImageExtractDir, imageIndexDir, releaseJSONPath}, "/")
		o.Log.Debug("List of all release artifacts in %s", artifactsDir)

		components, err := o.Manifest.GetReleaseSchema(artifactsDir)
		if err != nil {
			return err
		}
		for _, img := range components {
			o.Log.Info("\x1b[1;95m  %s \x1b[0m", img.Name)
		}
	}

	return nil
}
