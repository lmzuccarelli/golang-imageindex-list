package cli

import (
	"context"
	"fmt"
	"testing"

	clog "github.com/lmzuccarelli/golang-imageindex-list/pkg/log"
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
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
		}

		res := &cobra.Command{}
		res.SetContext(context.Background())
		res.SilenceUsage = true
		ex.Opts.Mode = "mirrorToDisk"
		err := ex.Run(res, []string{"oci://test"})
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
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
		ex.Opts.Mode = "mirrorToDisk"
		ex.Complete([]string{"operator"})
	})

	t.Run("Testing Executor : should fail (collector)", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: true}
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
		}

		res := &cobra.Command{}
		res.SilenceUsage = true
		res.SetContext(context.Background())
		err := ex.Run(res, []string{"operator"})
		if err == nil {
			t.Fatalf("should fail")
		}
	})

	/*
		t.Run("Testing Executor : should pass", func(t *testing.T) {
			opts.Global.Reference = "test-release-index:v0.0.1"
			res := NewCliCmd(log)
			res.SilenceUsage = true
			res.SetArgs([]string{"release"})
			err := res.Execute()
			if err != nil {
				log.Error(" %v ", err)
				t.Fatalf("should not fail")
			}
		})
	*/

	t.Run("Testing Executor : should pass", func(t *testing.T) {
		collector := &Collector{Log: log, Opts: opts, Fail: false}
		opts.Global.Reference = "test-operator-index:v0.0.1"
		ex := &ExecutorSchema{
			Log:       log,
			Opts:      opts,
			Collector: collector,
		}
		res := NewCliCmd(log)
		res.SilenceUsage = true
		err := ex.Run(res, []string{"operator"})
		if err != nil {
			log.Error(" %v ", err)
			t.Fatalf("should not fail")
		}
	})

	t.Run("Testing Executor : should fail", func(t *testing.T) {
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
}

// setup mocks

type Mirror struct{}

// for this test scenario we only need to mock
// ReleaseImageCollector, OperatorImageCollector and Batchr
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
