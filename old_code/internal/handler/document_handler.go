package handlers

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/arthurcommodore/cotapreco/cmd/worker"
	"github.com/arthurcommodore/cotapreco/internal/logic"
	"github.com/arthurcommodore/cotapreco/internal/logic/queue"
	"github.com/arthurcommodore/cotapreco/internal/model"
	"github.com/arthurcommodore/cotapreco/internal/repository"
	components "github.com/arthurcommodore/cotapreco/internal/template"
	"github.com/arthurcommodore/cotapreco/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var validate = validator.New()

func DocumentsComponent() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := c.MustGet("t").(func(string, ...string) string)

		userId, err := logic.GetUserIdByCookie(c)
		if err != nil {
			logic.Log("Error logic.GetUserIdByCookie(c), func DocumentsComponent", err)
			utils.JSONError(c,
				utils.StatusInternalServerError,
				map[string]any{"message": t("message_internal_error")},
			)
		}

		documents, err := logic.ListDocumentsByUserId(userId)
		if err != nil {
			logic.Log("Erro logic.ListProducts, func DocumentsComponent", err, userId)
			utils.JSONError(c,
				utils.StatusInternalServerError,
				map[string]any{"message": t("message_internal_error")},
			)
		}

		err = components.Documents(documents, t).Render(c.Request.Context(), c.Writer)
		if err != nil {
			logic.Log("Erro components.Documents, func DocumentsComponent", err, userId)
			utils.JSONError(c,
				utils.StatusInternalServerError,
				map[string]any{"message": t("message_internal_error")},
			)
		}
	}
}

// =======================
// UPLOAD + DISPARO OCR
// =======================
func UploadDocumentFile() gin.HandlerFunc {
	return func(c *gin.Context) {

		var fieldErrorsDocument = map[string]string{
			"Title": "message_title_required",
		}

		t := utils.AutoTranslator(c)

		user, err := logic.GetUserByCookie(c)
		if err != nil {
			logic.Log("Error logic.GetUserByCookie, func UploadDocumentFile", err)
			utils.JSONError(c, 500, gin.H{
				"message": t("message_internal_server_error"),
			})
			return
		}

		type uploadDocumentRequest struct {
			Title      string `json:"title" validate:"required"`
			Filename   string `json:"filename" validate:"required"`
			FileBase64 string `json:"fileBase64" validate:"required"`
		}

		var req uploadDocumentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logic.Log("Error c.ShouldBindJSON(&req), func UploadDocumentFile", err, user.ID.Hex())
			utils.JSON(c, http.StatusBadRequest, gin.H{
				"message": t("message_internal_server_error"),
			})
			return
		}

		if err := validate.Struct(req); err != nil {
			utils.JSONError(c, utils.StatusBadRequest, gin.H{
				"message": t(logic.GetFriendlyError(err, fieldErrorsDocument)),
			})
			return
		}

		fileBytes, err := base64.StdEncoding.DecodeString(req.FileBase64)
		// decodifica base64
		if err != nil {
			utils.JSONError(c, 500, gin.H{
				"message": t("message_invalid_file"),
			})
			return
		}

		const maxFileSize = 25 << 20 // 25MB
		if len(fileBytes) > maxFileSize {
			utils.JSON(c, http.StatusBadRequest, gin.H{
				"message": "message_file_too_large",
			})
			return
		}

		sampleSize := 512
		if len(fileBytes) < sampleSize {
			sampleSize = len(fileBytes)
		}

		contentType := http.DetectContentType(fileBytes[:sampleSize])
		if contentType != "application/pdf" {
			utils.JSON(c, http.StatusBadRequest, gin.H{
				"message": t("message_only_pdf"),
			})
			return
		}

		baseDir := "./static/documents"
		err = os.MkdirAll(baseDir, 0755)
		if err != nil {
			utils.JSONError(c, 500, gin.H{
				"message": t("message_invalid_file"),
			})
			return
		}

		filename := uuid.New().String() + ".pdf"
		fullPath := filepath.Join(baseDir, filename)

		if err := os.WriteFile(fullPath, fileBytes, 0644); err != nil {
			utils.JSONError(c, 500, gin.H{
				"message": t("message_internal_server_error"),
			})
			return
		}

		// cria document em processing
		document := model.Document{
			Title:  req.Title,
			UserId: user.ID.Hex(),
			Path:   fullPath,
			Time:   time.Now(),
		}

		insertResult, err := repository.DocumentRepo.Insert(document)
		if err != nil {
			utils.JSONError(c, 500, gin.H{
				"message": t("message_internal_error"),
			})
			return
		}

		documentObjectID, ok := insertResult.InsertedID.(primitive.ObjectID)
		if !ok {
			utils.JSONError(c, 500, gin.H{
				"message": "message_internal_error",
			})
			return
		}

		// dispara OCR
		err = queue.QueuePushStruct("files", worker.OCRFileQueueMessage{
			UserId:     user.ID.Hex(),
			Path:       fullPath,
			DocumentId: documentObjectID.Hex(),
		})

		if err != nil {
			logic.Log("Error queue.QueuePushStruct, func UploadDocumentFile", err, user.ID.Hex())
			utils.JSONError(c, 500, gin.H{
				"message": t("message_internal_error"),
			})
			logic.UpdateDocumentStatus(documentObjectID.Hex(), model.DocumentStatusFailed)
			return
		}

		utils.JSON(c, http.StatusAccepted, gin.H{
			"message": t("message_pdf_successfully_sent"),
		})
	}
}
