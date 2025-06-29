package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/git"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/auth"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/template"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/template/cache"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/template/render"

	_ "github.com/joho/godotenv/autoload"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlCache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/handler"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/integrations/helm"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/modulecontroller"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/prometheus"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/telemetry"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/cluster/k8sclient"

	cyclopsv1alpha1 "github.com/cyclops-ui/cyclops/cyclops-ctrl/api/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(cyclopsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8081", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8082", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("starting handler")

	telemetryClient, _ := telemetry.NewClient(
		getEnvBool("DISABLE_TELEMETRY"),
		os.Getenv("CYCLOPS_VERSION"),
		os.Getenv("INSTALL_MANAGER"),
		setupLog,
	)
	telemetryClient.InstanceStart()

	watchNamespace := getWatchNamespace()
	helmWatchNamespace := getHelmWatchNamespace()
	moduleTargetNamespace := getModuleTargetNamespace()

	k8sClient, err := k8sclient.New(
		watchNamespace,
		helmWatchNamespace,
		moduleTargetNamespace,
		zap.New(zap.UseFlagOptions(&opts)),
	)
	if err != nil {
		fmt.Println("error bootstrapping Kubernetes client", err)
		panic(err)
	}

	credsResolver := auth.NewTemplatesResolver(k8sClient)

	templatesRepo := template.NewRepo(
		credsResolver,
		cache.NewInMemoryTemplatesCache(),
	)

	monitor, err := prometheus.NewMonitor(setupLog)
	if err != nil {
		setupLog.Error(err, "failed to set up prom monitor")
	}

	renderer := render.NewRenderer(k8sClient)

	prometheus.StartCacheMetricsUpdater(&monitor, templatesRepo.ReturnCache(), 10*time.Second, setupLog)

	helmReleaseClient := helm.NewReleaseClient(helmWatchNamespace, k8sClient)
	gitWriteClient := git.NewWriteClient(credsResolver, getCommitMessageTemplate(), setupLog)

	handler, err := handler.New(templatesRepo, k8sClient, helmReleaseClient, renderer, gitWriteClient, moduleTargetNamespace, telemetryClient, monitor)
	if err != nil {
		panic(err)
	}

	go handler.Start()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "f9d9f115.cyclops-ui.com",
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		Cache: ctrlCache.Options{
			DefaultNamespaces: map[string]ctrlCache.Config{
				watchNamespace: {},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (modulecontroller.NewModuleReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		templatesRepo,
		k8sClient,
		renderer,
		getMaxConcurrentReconciles(),
		telemetryClient,
		monitor,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Module")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvBool(key string) bool {
	value := os.Getenv(key)
	if value == "" {
		return false
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return b
}

func getWatchNamespace() string {
	value := os.Getenv("WATCH_NAMESPACE")
	if value == "" {
		return "cyclops"
	}
	return value
}

func getModuleTargetNamespace() string {
	return os.Getenv("MODULE_TARGET_NAMESPACE")
}

func getHelmWatchNamespace() string {
	value := os.Getenv("WATCH_NAMESPACE_HELM")
	if value == "" {
		return ""
	}
	return value
}

func getMaxConcurrentReconciles() int {
	strValue := os.Getenv("MAX_CONCURRENT_RECONCILES")
	if strValue == "" {
		return 1
	}

	value, err := strconv.Atoi(strValue)
	if err != nil {
		return 1
	}

	return value
}

func getCommitMessageTemplate() string {
	return os.Getenv("COMMIT_MESSAGE_TEMPLATE")
}
