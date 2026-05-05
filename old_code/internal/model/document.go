package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

type Document struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Title       string             `bson:"title" json:"title" form:"title" validate:"required"`
	Path        string             `bson:"path" json:"path" form:"-"`
	Text        string             `bson:"text" json:"text"`
	CommentGPT  string             `bson:"commentGPT" json:"commentGPT"`
	UserId      string             `bson:"userId" json:"-"`
	Status      DocumentStatus     `bson:"status" json:"status"`
	LastUpdated time.Time          `bson:"lastUpdated" json:"lastUpdated"`
	Time        time.Time          `bson:"time" json:"time"`
}
