package routes

import (
	"github.com/arthurcommodore/cotapreco/internal/routes/handlers"
	"github.com/arthurcommodore/cotapreco/internal/utils"
	"github.com/gin-gonic/gin"
)

func DocumentActions(r *gin.RouterGroup) {
	utils.HandleTrailingSlash(r, r.POST, "/document_upload", handlers.UploadDocumentFile())
}

func DocumentPages(r *gin.RouterGroup) {
	utils.HandleTrailingSlash(r, r.GET, "/documents", handlers.DocumentsComponent())
}
