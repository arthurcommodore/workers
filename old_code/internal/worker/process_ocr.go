package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/arthurcommodore/cotapreco/internal/logic"
	"github.com/arthurcommodore/cotapreco/internal/logic/queue"
	"github.com/arthurcommodore/cotapreco/internal/model"
	"github.com/arthurcommodore/cotapreco/internal/realtime"
	"github.com/arthurcommodore/cotapreco/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OCRFileQueueMessage struct {
	UserId     string `json:"userId"`
	Path       string `json:"path"`
	DocumentId string `json:"documentId"`
}

func WorkerOCR() {
	fmt.Println("⚙️📄 OCR Worker ligado")
	ctxEvent := context.Background()

	for {
		var msg OCRFileQueueMessage
		evtSucces := realtime.Event{
			Type:    realtime.EventReloadContentAndNotification,
			Message: "message_file_processed",
		}
		evtError := realtime.Event{
			Type:    realtime.EventNotification,
			Message: "message_file_processing_error",
		}

		err := queue.QueuePopBlockingStruct("files", 0, &msg)
		if err != nil {
			logic.Log("Error QueuePopBlockingStruct files process_ocr.go", err)
			continue
		}

		if len(msg.Path) == 0 {
			logic.Log("Error len(msg.Path) == 0 process_ocr.go documentId:"+msg.DocumentId, msg.UserId, err)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			continue
		}

		path := msg.Path

		b, err := os.ReadFile(path)
		if err != nil {
			logic.Log("Error os.ReadFile(path) process_ocr.go documentId:"+msg.DocumentId, msg.UserId, err)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		txt, err := logic.ProcessUploadedFile(b)
		if err != nil {
			logic.Log("Erro OCR process_ocr.go documentId:"+msg.DocumentId, err, msg.UserId)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		documentID, err := primitive.ObjectIDFromHex(msg.DocumentId)
		if err != nil {
			logic.Log("primitive.ObjectIDFromHex process_ocr.go documentId:"+msg.DocumentId, msg.UserId, err)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		document, err := repository.DocumentRepo.FindOneAndUpsert(
			bson.M{"_id": documentID},
			repository.Partial[model.Document]{
				"text":   txt,
				"status": model.DocumentStatusCompleted,
			},
		)
		if err != nil {
			logic.Log("repository.DocumentRepo process_ocr.go documentId: "+msg.DocumentId, err, msg.UserId)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		user, err := logic.GetUserById(document.UserId)
		if err != nil {
			logic.Log("logic.GetUserById process_ocr.go documentId:"+msg.DocumentId, err, msg.UserId)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		data, err := json.Marshal(document)
		if err != nil {
			logic.Log("json.Marshal process_ocr.go documentId:"+msg.DocumentId, err, msg.UserId)
			realtime.ErrorEvent(ctxEvent, msg.UserId, evtError)
			logic.UpdateDocumentStatus(msg.DocumentId, model.DocumentStatusFailed)
			continue
		}

		realtime.UpdateEvent(&evtSucces, realtime.Event{
			UserId:  user.ID.Hex(),
			Type:    realtime.EventReloadContentAndNotification,
			Lang:    user.Lang,
			Route:   "/documents",
			Payload: data,
		})

		realtime.SuccessEvent(ctxEvent, msg.UserId, evtSucces)
	}
}
