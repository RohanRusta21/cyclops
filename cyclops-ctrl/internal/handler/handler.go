package handler

import (
	"net/http"

	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/controller/sse"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/controller/ws"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/git"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/integrations/helm"
	templaterepo "github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/template"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/template/render"
	"github.com/gin-gonic/gin"

	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/controller"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/prometheus"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/internal/telemetry"
	"github.com/cyclops-ui/cyclops/cyclops-ctrl/pkg/cluster/k8sclient"
)

type Handler struct {
	router *gin.Engine

	templatesRepo  templaterepo.ITemplateRepo
	k8sClient      k8sclient.IKubernetesClient
	releaseClient  *helm.ReleaseClient
	renderer       *render.Renderer
	gitWriteClient *git.WriteClient

	moduleTargetNamespace string

	telemetryClient telemetry.Client
	monitor         prometheus.Monitor
}

func New(
	templatesRepo templaterepo.ITemplateRepo,
	kubernetesClient k8sclient.IKubernetesClient,
	releaseClient *helm.ReleaseClient,
	renderer *render.Renderer,
	gitWriteClient *git.WriteClient,
	moduleTargetNamespace string,
	telemetryClient telemetry.Client,
	monitor prometheus.Monitor,
) (*Handler, error) {
	return &Handler{
		templatesRepo:         templatesRepo,
		k8sClient:             kubernetesClient,
		renderer:              renderer,
		releaseClient:         releaseClient,
		gitWriteClient:        gitWriteClient,
		moduleTargetNamespace: moduleTargetNamespace,
		telemetryClient:       telemetryClient,
		monitor:               monitor,
	}, nil
}

func (h *Handler) Start() error {
	gin.SetMode(gin.DebugMode)

	templatesController := controller.NewTemplatesController(h.templatesRepo, h.k8sClient, h.telemetryClient)
	modulesController := controller.NewModulesController(h.templatesRepo, h.k8sClient, h.renderer, h.gitWriteClient, h.moduleTargetNamespace, h.telemetryClient, h.monitor)
	clusterController := controller.NewClusterController(h.k8sClient)
	helmController := controller.NewHelmController(h.k8sClient, h.releaseClient, h.telemetryClient)

	h.router = gin.New()

	server := sse.NewServer(h.k8sClient, h.releaseClient)
	wsServer := ws.NewServer(h.k8sClient)

	h.router.GET("/exec/:podNamespace/:podName/:containerName", wsServer.ExecCommand)

	h.router.GET("/stream/resources/:name", sse.HeadersMiddleware(), server.Resources)
	h.router.GET("/stream/releases/:namespace/:name/resources", sse.HeadersMiddleware(), server.ReleaseResources)
	h.router.POST("/stream/resources", sse.HeadersMiddleware(), server.SingleResource)

	h.router.GET("/ping", h.pong())

	// templates
	h.router.GET("/templates", templatesController.GetTemplate)
	h.router.GET("/templates/initial", templatesController.GetTemplateInitialValues)

	h.router.GET("/templates/revisions", templatesController.GetTemplateRevisions)

	// templates store
	h.router.GET("/templates/store", templatesController.ListTemplatesStore)
	h.router.PUT("/templates/store", templatesController.CreateTemplatesStore)
	h.router.POST("/templates/store/:name", templatesController.EditTemplatesStore)
	h.router.DELETE("/templates/store/:name", templatesController.DeleteTemplatesStore)

	// modules
	h.router.GET("/modules/:name", modulesController.GetModule)
	h.router.GET("/modules/list", modulesController.ListModules)
	h.router.DELETE("/modules/:name", modulesController.DeleteModule)
	h.router.POST("/modules/new", modulesController.CreateModule)
	h.router.POST("/modules/update", modulesController.UpdateModule)
	h.router.POST("/modules/rollback/manifest", modulesController.HistoryEntryManifest)
	h.router.POST("/modules/rollback", modulesController.RollbackModule)
	h.router.GET("/modules/:name/raw", modulesController.GetRawModuleManifest)
	h.router.POST("/modules/:name/reconcile", modulesController.ReconcileModule)
	h.router.GET("/modules/:name/history", modulesController.GetModuleHistory)
	h.router.POST("/modules/:name/manifest", modulesController.Manifest)
	h.router.GET("/modules/:name/currentManifest", modulesController.CurrentManifest)
	h.router.GET("/modules/:name/resources", modulesController.ResourcesForModule)
	h.router.GET("/modules/:name/template", modulesController.Template)
	h.router.GET("/modules/:name/helm-template", modulesController.HelmTemplate)
	//h.router.POST("/modules/resources", modulesController.ModuleToResources)

	h.router.POST("/modules/mcp/install", modulesController.InstallMCPServer)
	h.router.GET("/modules/mcp/status", modulesController.MCPServerStatus)

	h.router.GET("/resources/pods/:namespace/:name/:container/logs", modulesController.GetLogs)
	h.router.GET("/resources/pods/:namespace/:name/:container/logs/stream", sse.HeadersMiddleware(), modulesController.GetLogsStream)
	h.router.GET("/resources/pods/:namespace/:name/:container/logs/download", modulesController.DownloadLogs)

	h.router.GET("/manifest", modulesController.GetManifest)
	h.router.GET("/resources", modulesController.GetResource)
	h.router.DELETE("/resources", modulesController.DeleteModuleResource)

	h.router.POST("/resources/restart", modulesController.Restart)

	h.router.GET("/nodes", clusterController.ListNodes)
	h.router.GET("/nodes/:name", clusterController.GetNode)

	h.router.GET("/namespaces", clusterController.ListNamespaces)

	// region helm migrator
	h.router.GET("/helm/releases", helmController.ListReleases)
	h.router.GET("/helm/releases/:namespace/:name", helmController.GetRelease)
	h.router.POST("/helm/releases/:namespace/:name", helmController.UpgradeRelease)
	h.router.DELETE("/helm/releases/:namespace/:name", helmController.UninstallRelease)
	h.router.GET("/helm/releases/:namespace/:name/resources", helmController.GetReleaseResources)
	h.router.GET("/helm/releases/:namespace/:name/fields", helmController.GetReleaseSchema)
	h.router.GET("/helm/releases/:namespace/:name/values", helmController.GetReleaseValues)
	h.router.POST("/helm/releases/:namespace/:name/migrate", helmController.MigrateHelmRelease)
	// endregion

	h.router.Use(h.options)

	return h.router.Run()
}

func (h *Handler) pong() func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "pong")
	}
}

func (h *Handler) options(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	if ctx.Request.Method != http.MethodOptions {
		ctx.Next()
		return
	}

	ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
	ctx.Header("Access-Control-Allow-Headers", "authorization, origin, content-type, accept")
	ctx.Header("Allow", "HEAD,GET,POST,PUT,PATCH,DELETE,OPTIONS")
	ctx.Header("Content-Type", "application/json")
	ctx.AbortWithStatus(http.StatusOK)
}
