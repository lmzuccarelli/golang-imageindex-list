package manifest

import (
	"testing"

	"github.com/lmzuccarelli/golang-imageindex-list/pkg/api/v1alpha3"
	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
)

func TestGetAllManifests(t *testing.T) {

	log := clog.New("debug")

	// these tests should cover over 80%
	t.Run("Testing GetImageIndex : should pass", func(t *testing.T) {
		manifest := &Manifest{Log: log}
		res, err := manifest.GetImageIndex("../../tests/")
		if err != nil {
			t.Fatalf("should not fail")
		}
		log.Debug("completed test  %v ", res)
	})

	t.Run("Testing GetImageManifest : should pass", func(t *testing.T) {
		manifest := &Manifest{Log: log}
		res, err := manifest.GetImageManifest("../../tests/image-manifest.json")
		if err != nil {
			t.Fatalf("should not fail")
		}
		log.Debug("completed test  %v ", res)
	})

	t.Run("Testing GetOperatorConfig : should pass", func(t *testing.T) {
		manifest := &Manifest{Log: log}
		res, err := manifest.GetOperatorConfig("../../tests/operator-config.json")
		if err != nil {
			t.Fatalf("should not fail")
		}
		log.Debug("completed test  %v ", res)
	})

	t.Run("Testing GetReleaseSchema : should pass", func(t *testing.T) {
		manifest := &Manifest{Log: log}
		res, err := manifest.GetReleaseSchema("../../tests/release-schema.json")
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
		log.Debug("completed test  %v ", res)
	})

}

func TestExtractOCILayers(t *testing.T) {

	log := clog.New("debug")
	t.Run("Testing ExtractOCILayers : should pass", func(t *testing.T) {
		oci := &v1alpha3.OCISchema{
			SchemaVersion: 2,
			Manifests: []v1alpha3.OCIManifest{
				{
					MediaType: "application/vnd.oci.image.manifest.v1+json",
					Digest:    "sha256:3ef0b0141abd1548f60c4f3b23ecfc415142b0e842215f38e98610a3b2e52419",
					Size:      567,
				},
			},
			Config: v1alpha3.OCIManifest{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:3ef0b0141abd1548f60c4f3b23ecfc415142b0e842215f38e98610a3b2e52419",
				Size:      567,
			},
			Layers: []v1alpha3.OCIManifest{
				{
					MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
					Digest:    "sha256:d8190195889efb5333eeec18af9b6c82313edd4db62989bd3a357caca4f13f0e",
					Size:      1438,
				},
				{
					MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
					Digest:    "sha256:5b2ca04f694b70c8b41f1c2a40b7e95643181a1d037b115149ecc243324c513d",
					Size:      955593,
				},
			},
		}
		manifest := &Manifest{Log: log}
		err := manifest.ExtractLayersOCI("../../tests/test-untar/blobs/sha256", "../../tests/hold-test-untar", "release-manifests/", oci)
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
	})
}

func TestListCatalogs(t *testing.T) {

	log := clog.New("debug")
	t.Run("Testing ListCatalogs : should pass", func(t *testing.T) {
		manifest := New(log)
		err := manifest.ListCatalogs("../../tests/configs", "3scale-operator")
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
	})
}
