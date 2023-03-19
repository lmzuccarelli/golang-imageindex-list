package manifest

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lmzuccarelli/golang-imageindex-list/pkg/api/v1alpha3"
	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
)

const (
	index                   string = "index.json"
	catalogJson             string = "catalog.json"
	operatorImageExtractDir string = "hold-operator"
)

type ManifestInterface interface {
	GetImageIndex(dir string) (*v1alpha3.OCISchema, error)
	GetImageManifest(file string) (*v1alpha3.OCISchema, error)
	GetOperatorConfig(file string) (*v1alpha3.OperatorConfigSchema, error)
	ExtractLayersOCI(filePath, toPath, label string, oci *v1alpha3.OCISchema) error
	GetReleaseSchema(filePath string) ([]v1alpha3.RelatedImage, error)
	ListCatalogs(filePath, op string) error
}

type Manifest struct {
	Log clog.PluggableLoggerInterface
}

func New(log clog.PluggableLoggerInterface) ManifestInterface {
	return &Manifest{Log: log}
}

// GetImageIndex - used to get the oci index.json
func (o *Manifest) GetImageIndex(dir string) (*v1alpha3.OCISchema, error) {
	var oci *v1alpha3.OCISchema
	indx, err := os.ReadFile(dir + "/" + index)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(indx, &oci)
	if err != nil {
		return nil, err
	}
	return oci, nil
}

// GetImageManifest used to ge the manifest in the oci blobs/sha254
// directory - found in index.json
func (o *Manifest) GetImageManifest(file string) (*v1alpha3.OCISchema, error) {
	var oci *v1alpha3.OCISchema
	manifest, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(manifest, &oci)
	if err != nil {
		return nil, err
	}
	return oci, nil
}

// GetOperatorConfig used to parse the operator json
func (o *Manifest) GetOperatorConfig(file string) (*v1alpha3.OperatorConfigSchema, error) {
	var ocs *v1alpha3.OperatorConfigSchema
	manifest, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(manifest, &ocs)
	if err != nil {
		return nil, err
	}
	return ocs, nil
}

// ExtractLayersOCI
func (o *Manifest) ExtractLayersOCI(fromPath, toPath, label string, oci *v1alpha3.OCISchema) error {
	// first check if the labelled artifact is untarred
	// no use to untar it if its already untarred
	if _, err := os.Stat(toPath + label); errors.Is(err, os.ErrNotExist) {
		for _, blob := range oci.Layers {
			if !strings.Contains(blob.Digest, "sha256") {
				return fmt.Errorf("the digest format is not correct %s ", blob.Digest)
			}
			f, err := os.Open(fromPath + "/" + strings.Split(blob.Digest, ":")[1])
			if err != nil {
				return err
			}
			err = untar(f, toPath, label)
			if err != nil {
				return err
			}
		}
	} else {
		o.Log.Info("tar exists")
	}
	return nil
}

// GetReleaseSchema
func (o *Manifest) GetReleaseSchema(filePath string) ([]v1alpha3.RelatedImage, error) {
	var release = v1alpha3.ReleaseSchema{}

	file, _ := os.ReadFile(filePath)
	err := json.Unmarshal([]byte(file), &release)
	if err != nil {
		return []v1alpha3.RelatedImage{}, err
	}

	var allImages []v1alpha3.RelatedImage
	for _, item := range release.Spec.Tags {
		allImages = append(allImages, v1alpha3.RelatedImage{Image: item.From.Name, Name: item.Name})
	}
	return allImages, nil
}

// ListCatalogs
func (o *Manifest) ListCatalogs(filePath string, op string) error {
	// the catalog.json - does not really conform to json standards
	// this needs some thorough testing
	olm, err := readOperatorCatalog(filePath + "/" + op)
	if err != nil {
		return err
	}
	// iterate through the catalog objects
	for _, obj := range olm {
		switch {
		case obj.Schema == "olm.channel":
			o.Log.Info("\x1b[1;92m   channel => %s \x1b[0m", obj.Name)
			for _, e := range obj.Entries {
				o.Log.Info("\x1b[1;94m     name: %s \x1b[0m", e.Name)
				if len(e.Replaces) > 0 {
					o.Log.Info("       replaces: %s", e.Replaces)
				}
				if len(e.Skips) > 0 {
					for _, x := range e.Skips {
						o.Log.Info("       skips: %s", x)
					}
				}
				if len(e.SkipRange) > 0 {
					o.Log.Info("       skiprange: %s", e.SkipRange)
				}
			}
		case obj.Schema == "olm.bundle":
			o.Log.Info("\x1b[1;92m   bundle: %s \x1b[0m", obj.Name)
			//o.Log.Trace("config relatedImages: %v", obj.RelatedImages)
		}
	}
	return nil
}

// utility / helper functions

// UntarLayers simple function that untars the image layers
func untar(gzipStream io.Reader, path string, cfgDirName string) error {
	//Remove any separators in cfgDirName as received from the label
	cfgDirName = strings.TrimSuffix(cfgDirName, "/")
	cfgDirName = strings.TrimPrefix(cfgDirName, "/")
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return fmt.Errorf("untar: gzipStream - %w", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("untar: Next() failed: %s", err.Error())
		}

		if strings.Contains(header.Name, cfgDirName) {
			switch header.Typeflag {
			case tar.TypeDir:
				if header.Name != "./" {
					if err := os.MkdirAll(path+"/"+header.Name, 0755); err != nil {
						return fmt.Errorf("untar: Mkdir() failed: %v", err)
					}
				}
			case tar.TypeReg:
				outFile, err := os.Create(path + "/" + header.Name)
				if err != nil {
					return fmt.Errorf("untar: Create() failed: %v", err)
				}
				if _, err := io.Copy(outFile, tarReader); err != nil {
					return fmt.Errorf("untar: Copy() failed: %v", err)
				}
				outFile.Close()

			default:
				// just ignore errors as we are only interested in the FB configs layer
				fmt.Printf("WARNING : untar: unknown type: %v in %s\n", header.Typeflag, header.Name)
			}
		}
	}
	return nil
}

// readOperatorCatalog - simple function tha treads the specific catalog.json file
// and unmarshals it to DeclarativeConfig struct
func readOperatorCatalog(path string) ([]v1alpha3.DeclarativeConfig, error) {
	// the catalog.json - dos not really conform to json standards
	var olm []v1alpha3.DeclarativeConfig
	data, err := os.ReadFile(path + "/" + catalogJson)
	if err != nil {
		return []v1alpha3.DeclarativeConfig{}, err
	}
	tmp := strings.NewReplacer(" ", "").Replace(string(data))
	updatedJson := "[" + strings.ReplaceAll(tmp, "}\n{", "},{") + "]"
	err = json.Unmarshal([]byte(updatedJson), &olm)
	if err != nil {
		return []v1alpha3.DeclarativeConfig{}, err
	}
	return olm, nil
}
