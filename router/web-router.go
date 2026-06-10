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

// ThemeAssets holds the embedded frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
	ClassicBuildFS   embed.FS
	ClassicIndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")
	themeFS := common.NewThemeAwareFS(defaultFS, classicFS)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	gptLoad := router.Group("/gl")
	gptLoad.Use(middleware.RootSessionAuth())
	{
		gptLoad.Any("", controller.GPTLoadProxy)
		gptLoad.Any("/*path", controller.GPTLoadProxy)
	}
	cpaManagerPlus := router.Group("/cpa")
	cpaManagerPlus.Use(middleware.RootSessionAuth())
	{
		cpaManagerPlus.Any("", controller.CPAManagerPlusProxy)
		cpaManagerPlus.Any("/*path", controller.CPAManagerPlusProxy)
	}
	cliProxyAPI := router.Group("/cpa-native")
	cliProxyAPI.Use(middleware.RootSessionAuth())
	{
		cliProxyAPI.Any("", controller.CLIProxyAPIProxy)
		cliProxyAPI.Any("/*path", controller.CLIProxyAPIProxy)
	}
	cpaManagerPlusRoot := router.Group("")
	cpaManagerPlusRoot.Use(middleware.RootSessionAuth())
	{
		cpaManagerPlusRoot.Any("/usage-service/*path", controller.CPAManagerPlusRootProxy)
		cpaManagerPlusRoot.Any("/v0/management/*path", controller.CPAManagerPlusRootProxy)
		cpaManagerPlusRoot.Any("/status", controller.CPAManagerPlusRootProxy)
		cpaManagerPlusRoot.Any("/setup", controller.CPAManagerPlusRootProxy)
	}
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", themeFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		if common.GetTheme() == "classic" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}
