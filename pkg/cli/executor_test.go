package cli

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/manifest"
	"github.com/lmzuccarelli/golang-imageindex-list/pkg/mirror"
	"github.com/spf13/cobra"
)

func TestExecutor(t *testing.T) {

	log := clog.New("info")

	global := &mirror.GlobalOptions{
		TlsVerify:      false,
		InsecurePolicy: true,
		Force:          true,
		Dir:            "tests",
	}
	_, sharedOpts := mirror.SharedImageFlags()
	_, deprecatedTLSVerifyOpt := mirror.DeprecatedTLSVerifyFlags()
	_, srcOpts := mirror.ImageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	_, destOpts := mirror.ImageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	_, retryOpts := mirror.RetryFlags()

	opts := mirror.CopyOptions{
		Global:              global,
		DeprecatedTLSVerify: deprecatedTLSVerifyOpt,
		SrcImage:            srcOpts,
		DestImage:           destOpts,
		RetryOpts:           retryOpts,
		Dev:                 false,
	}

	// this test should cover over 80%
	t.Run("Testing Executor : should pass", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		manifest := manifest.New(log)
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Manifest:  manifest,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, false, false, false}},
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		err := ex.Run(res, []string{"oci://test"})
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
	})

	t.Run("Testing Executor : should fail (mkdir[0] forced error)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{true, false, false, false}},
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		err := ex.Run(res, []string{"oci://test"})
		if err == nil {
			log.Error(" %v ", err)
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Executor : should fail (mkdir[1] forced error)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, true, false, false}},
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		err := ex.Run(res, []string{"oci://test"})
		if err == nil {
			log.Error(" %v ", err)
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Executor : should fail (mkdir[2] forced error)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, false, true, false}},
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		err := ex.Run(res, []string{"oci://test"})
		if err == nil {
			log.Error(" %v ", err)
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Executor : should fail (mkdir[3] forced error)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, false, false, true}},
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		err := ex.Run(res, []string{"oci://test"})
		if err == nil {
			log.Error(" %v ", err)
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Executor : should pass", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		ex.Complete([]string{"operator"})
	})

	t.Run("Testing Executor : should fail (collector)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: true}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, false, false, false}},
		}

		res := &cobra.Command{}
		res.SilenceUsage = true
		res.SetContext(context.Background())
		err := ex.Run(res, []string{"operator"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Executor : should pass", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		opts.Global.Reference = "test-operator-index:v0.0.1"
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
			Dir:       &mockMakeDirectory{Fail: [4]bool{false, false, false, false}},
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Run(res, []string{"operator"})
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
	})

	t.Run("Testing Validate : should fail", func(t *testing.T) {
		ex := &ExecutorSchema{
			Log:  log,
			Opts: opts,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Validate([]string{"test"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Validate : should fail", func(t *testing.T) {
		opts.Global.Reference = "test-operator-index:v0.0.1"
		ex := &ExecutorSchema{
			Log:  log,
			Opts: opts,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Validate([]string{"release"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Validate : should fail", func(t *testing.T) {
		opts.Global.Reference = "test-release-index:v0.0.1"
		ex := &ExecutorSchema{
			Log:  log,
			Opts: opts,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Validate([]string{"operator"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Validate : should fail", func(t *testing.T) {
		opts.Global.Reference = ""
		ex := &ExecutorSchema{
			Log:  log,
			Opts: opts,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Validate([]string{"operator"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	t.Run("Testing Validate : should pass", func(t *testing.T) {
		opts.Global.Reference = "test-release-index:v0.0.1"
		ex := &ExecutorSchema{
			Log:  log,
			Opts: opts,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Validate([]string{"release"})
		if err != nil {
			t.Fatalf("should not fail")
		}
	})

}

// setup mocks

type Mirror struct{}

type mockMakeDirectory struct {
	Fail [4]bool
}

// for this test scenario we only need to mock
// ReleaseImageCollector, OperatorImageCollector and Batch
type Collector struct {
	Log  clog.PluggableLoggerInterface
	Opts mirror.CopyOptions
	Fail bool
}

func (o *Collector) Collect() error {
	if o.Fail {
		return fmt.Errorf("forced error operator collector")
	}
	return nil
}

func (o *mockMakeDirectory) MakeDir(dir string, mode fs.FileMode) error {
	switch {
	case strings.Contains(dir, "working") && o.Fail[0]:
		return fmt.Errorf("forced create working directory error")
	case strings.Contains(dir, "logs") && o.Fail[1]:
		return fmt.Errorf("forced create logs directory error")
	case strings.Contains(dir, "operator") && o.Fail[2]:
		return fmt.Errorf("forced create operator directory error")
	case strings.Contains(dir, "release") && o.Fail[3]:
		return fmt.Errorf("forced create release directory error")
	}
	return nil
}
