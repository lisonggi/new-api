package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets, pluginDispatcher gin.HandlerFunc) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")

	router.NoRoute(
		pluginDispatcher,
		middleware.RouteTag("web"),
		gzip.Gzip(gzip.DefaultCompression),
		middleware.AccessTokenAudit(),
		middleware.GlobalWebRateLimit(),
		middleware.Cache(),
		static.Serve("/", frontendFS),
		func(c *gin.Context) {
			if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
				controller.RelayNotFound(c)
				return
			}
			// Serve a build-time pre-rendered static page when one exists for the
			// requested path (e.g. web/dist/about.html for /about). embed.FS rejects
			// path traversal, so joining the request path is safe.
			if c.Request.Method == http.MethodGet {
				cleanPath := strings.TrimSuffix(c.Request.URL.Path, "/")
				if data, err := assets.BuildFS.ReadFile("web/dist" + cleanPath + ".html"); err == nil {
					c.Header("Cache-Control", middleware.CacheControlHTML)
					c.Data(http.StatusOK, "text/html; charset=utf-8", data)
					return
				}
			}
			c.Header("Cache-Control", middleware.CacheControlHTML)
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.IndexPage)
		},
	)
}
