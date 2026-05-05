package logic

import (
	"github.com/arthurcommodore/cotapreco/internal/model"
	"github.com/arthurcommodore/cotapreco/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func SaveDocument(document model.Document) error {
	_, err := repository.DocumentRepo.Insert(document)
	if err != nil {
		Log("Error DocumentRepo.Insert, func SaveDocument", err, document.UserId)
		return err
	}
	return nil
}

func ListDocumentsByUserId(userId string) ([]model.Document, error) {

	var documents []model.Document

	documents, err := repository.DocumentRepo.FindAll(bson.M{"userId": userId})
	if err != nil {
		Log("Error repository.DocumentRepo.FindAll, func ListDocumentsByUserId", err, userId)
		return documents, err
	}

	return documents, nil
}

func UpdateDocumentStatus(documentId string, status model.DocumentStatus) (*model.Document, error) {

	objId, err := primitive.ObjectIDFromHex(documentId)
	if err != nil {
		Log("Error primitive.ObjectIDFromHex, func UpdateDocumentStatus, documentId:" + documentId)
		return nil, err
	}

	document, err := repository.DocumentRepo.FindOneAndUpsert(
		bson.M{"_id": objId},
		repository.Partial[model.Document]{
			"status": status,
		},
	)

	if err != nil {
		Log("Error repository.DocumentRepo.FindOneAndUpsert, func UpdateDocumentStatus documentId: "+documentId, err)
		return nil, err
	}

	return document, nil
}
