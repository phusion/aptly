package api

import (
	"net/http"

	ctx "github.com/aptly-dev/aptly/context"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var context *ctx.AptlyContext

func apiMetricsGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		promhttp.Handler().ServeHTTP(c.Writer, c.Request)
	}
}

// Router returns prebuilt with routes http.Handler
func Router(c *ctx.AptlyContext) http.Handler {
	context = c

	router := gin.Default()
	router.UseRawPath = true
	router.Use(gin.ErrorLogger())

	if c.Config().EnableMetricsEndpoint {
		router.Use(instrumentHandlerInFlight(apiRequestsInFlightGauge, getBasePath))
		router.Use(instrumentHandlerCounter(apiRequestsTotalCounter, getBasePath))
		router.Use(instrumentHandlerRequestSize(apiRequestSizeSummary, getBasePath))
		router.Use(instrumentHandlerResponseSize(apiResponseSizeSummary, getBasePath))
		router.Use(instrumentHandlerDuration(apiRequestsDurationSummary, getBasePath))
	}

	if context.Flags().Lookup("no-lock").Value.Get().(bool) {
		// We use a goroutine to count the number of
		// concurrent requests. When no more requests are
		// running, we close the database to free the lock.
		dbRequests = make(chan dbRequest)

		go acquireDatabase()

		router.Use(func(c *gin.Context) {
			var err error

			errCh := make(chan error)
			dbRequests <- dbRequest{acquiredb, errCh}

			err = <-errCh
			if err != nil {
				c.AbortWithError(500, err)
				return
			}

			defer func() {
				dbRequests <- dbRequest{releasedb, errCh}
				err = <-errCh
				if err != nil {
					c.AbortWithError(500, err)
				}
			}()

			c.Next()
		})
	}

	root := router.Group("/api")

	{
		if c.Config().EnableMetricsEndpoint {
			root.GET("/metrics", apiMetricsGet())
		}
		root.GET("/version", apiVersion)
	}

	{
		root.GET("/repos", apiReposList)
		root.POST("/repos", apiReposCreate)
		root.GET("/repos/:name", apiReposShow)
		root.PUT("/repos/:name", apiReposEdit)
		root.DELETE("/repos/:name", apiReposDrop)

		root.GET("/repos/:name/packages", apiReposPackagesShow)
		root.POST("/repos/:name/packages", apiReposPackagesAdd)
		root.DELETE("/repos/:name/packages", apiReposPackagesDelete)

		root.POST("/repos/:name/file/:dir/:file", apiReposPackageFromFile)
		root.POST("/repos/:name/file/:dir", apiReposPackageFromDir)

		root.POST("/repos/:name/include/:dir/:file", apiReposIncludePackageFromFile)
		root.POST("/repos/:name/include/:dir", apiReposIncludePackageFromDir)

		root.POST("/repos/:name/snapshots", apiSnapshotsCreateFromRepository)
	}

	{
		root.POST("/mirrors/:name/snapshots", apiSnapshotsCreateFromMirror)
	}

	{
		root.GET("/mirrors", apiMirrorsList)
		root.GET("/mirrors/:name", apiMirrorsShow)
		root.GET("/mirrors/:name/packages", apiMirrorsPackages)
		root.POST("/mirrors", apiMirrorsCreate)
		root.PUT("/mirrors/:name", apiMirrorsUpdate)
		root.DELETE("/mirrors/:name", apiMirrorsDrop)
	}

	{
		root.POST("/gpg/key", apiGPGAddKey)
	}

	{
		root.GET("/files", apiFilesListDirs)
		root.POST("/files/:dir", apiFilesUpload)
		root.GET("/files/:dir", apiFilesListFiles)
		root.DELETE("/files/:dir", apiFilesDeleteDir)
		root.DELETE("/files/:dir/:name", apiFilesDeleteFile)
	}

	{
		root.GET("/publish", apiPublishList)
		root.POST("/publish", apiPublishRepoOrSnapshot)
		root.POST("/publish/:prefix", apiPublishRepoOrSnapshot)
		root.PUT("/publish/:prefix/:distribution", apiPublishUpdateSwitch)
		root.DELETE("/publish/:prefix/:distribution", apiPublishDrop)
	}

	{
		root.GET("/snapshots", apiSnapshotsList)
		root.POST("/snapshots", apiSnapshotsCreate)
		root.PUT("/snapshots/:name", apiSnapshotsUpdate)
		root.GET("/snapshots/:name", apiSnapshotsShow)
		root.GET("/snapshots/:name/packages", apiSnapshotsSearchPackages)
		root.DELETE("/snapshots/:name", apiSnapshotsDrop)
		root.GET("/snapshots/:name/diff/:withSnapshot", apiSnapshotsDiff)
	}

	{
		root.GET("/packages/:key", apiPackagesShow)
		root.GET("/packages", apiPackages)
	}

	{
		root.GET("/graph.:ext", apiGraph)
	}
	{
		root.POST("/db/cleanup", apiDbCleanup)
	}
	{
		root.GET("/tasks", apiTasksList)
		root.POST("/tasks-clear", apiTasksClear)
		root.GET("/tasks-wait", apiTasksWait)
		root.GET("/tasks/:id/wait", apiTasksWaitForTaskByID)
		root.GET("/tasks/:id/output", apiTasksOutputShow)
		root.GET("/tasks/:id/detail", apiTasksDetailShow)
		root.GET("/tasks/:id/return_value", apiTasksReturnValueShow)
		root.GET("/tasks/:id", apiTasksShow)
		root.DELETE("/tasks/:id", apiTasksDelete)
		root.POST("/tasks-dummy", apiTasksDummy)
	}

	return router
}
