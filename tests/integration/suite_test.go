package integration

import (
	"context"
	"log"
	"log/slog"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/virtualip-manager/cmd/config"
	"github.com/scality/virtualip-manager/pkg/infrastructure/di"
	// +kubebuilder:scaffold:imports
)

type TestingSuite struct {
	logger    *slog.Logger
	container *di.Container
}

var (
	ctx          context.Context
	cancel       context.CancelFunc
	testingSuite *TestingSuite
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "VirtualIP Manager Suite")
}

var _ = BeforeSuite(func() {
	handler := slog.NewJSONHandler(GinkgoWriter, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	os.Setenv("NODE_NAME", "bootstrap") //nolint:errcheck
	os.Setenv("NODE_IP", "1.1.1.1")     //nolint:errcheck
	cfg, err := config.NewEnvironment(ctx, "")
	if err != nil {
		log.Fatal(err) //nolint:revive // This is basically the main function, shut up revive
	}
	container := di.NewContainer(cfg)
	container.GetMockInterfaceGetter()
	testingSuite = &TestingSuite{
		logger:    logger,
		container: container,
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
})
