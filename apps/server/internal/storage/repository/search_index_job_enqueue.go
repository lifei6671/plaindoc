package repository

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

type searchIndexDocumentPayload struct {
	DocumentID string `json:"documentId"`
}

type searchIndexSpacePayload struct {
	SpaceID string `json:"spaceId"`
}

// BuildSearchIndexDocUpsertJob 构建文档 upsert 任务入队参数。
func BuildSearchIndexDocUpsertJob(documentID string) (EnqueueSearchIndexJobParams, error) {
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return EnqueueSearchIndexJobParams{}, errors.New("search index document id is empty")
	}
	payloadJSON, err := encodeSearchIndexDocumentPayload(normalizedDocumentID)
	if err != nil {
		return EnqueueSearchIndexJobParams{}, err
	}
	return EnqueueSearchIndexJobParams{
		JobType:     models.SearchIndexJobTypeDocUpsert,
		DedupeKey:   "doc:" + normalizedDocumentID,
		PayloadJSON: payloadJSON,
		Priority:    models.SearchIndexJobPriorityNormal,
		NextRunAt:   time.Now().UTC(),
	}, nil
}

// BuildSearchIndexDocDeleteJob 构建文档 delete 任务入队参数。
func BuildSearchIndexDocDeleteJob(documentID string) (EnqueueSearchIndexJobParams, error) {
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedDocumentID == "" {
		return EnqueueSearchIndexJobParams{}, errors.New("search index document id is empty")
	}
	payloadJSON, err := encodeSearchIndexDocumentPayload(normalizedDocumentID)
	if err != nil {
		return EnqueueSearchIndexJobParams{}, err
	}
	return EnqueueSearchIndexJobParams{
		JobType:     models.SearchIndexJobTypeDocDelete,
		DedupeKey:   "doc:" + normalizedDocumentID,
		PayloadJSON: payloadJSON,
		Priority:    models.SearchIndexJobPriorityHigh,
		NextRunAt:   time.Now().UTC(),
	}, nil
}

// BuildSearchIndexSpacePurgeJob 构建空间 purge 任务入队参数。
func BuildSearchIndexSpacePurgeJob(spaceID string) (EnqueueSearchIndexJobParams, error) {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return EnqueueSearchIndexJobParams{}, errors.New("search index space id is empty")
	}
	payloadJSON, err := encodeSearchIndexSpacePayload(normalizedSpaceID)
	if err != nil {
		return EnqueueSearchIndexJobParams{}, err
	}
	return EnqueueSearchIndexJobParams{
		JobType:     models.SearchIndexJobTypeSpacePurge,
		DedupeKey:   "space:" + normalizedSpaceID,
		PayloadJSON: payloadJSON,
		Priority:    models.SearchIndexJobPriorityHigh,
		NextRunAt:   time.Now().UTC(),
	}, nil
}

// BuildSearchIndexRebuildSpaceJob 构建空间 rebuild 任务入队参数。
func BuildSearchIndexRebuildSpaceJob(spaceID string) (EnqueueSearchIndexJobParams, error) {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID == "" {
		return EnqueueSearchIndexJobParams{}, errors.New("search index space id is empty")
	}
	payloadJSON, err := encodeSearchIndexSpacePayload(normalizedSpaceID)
	if err != nil {
		return EnqueueSearchIndexJobParams{}, err
	}
	return EnqueueSearchIndexJobParams{
		JobType:     models.SearchIndexJobTypeRebuildSpace,
		DedupeKey:   "space:" + normalizedSpaceID,
		PayloadJSON: payloadJSON,
		Priority:    models.SearchIndexJobPriorityNormal,
		NextRunAt:   time.Now().UTC(),
	}, nil
}

func encodeSearchIndexDocumentPayload(documentID string) (string, error) {
	value, err := json.Marshal(searchIndexDocumentPayload{
		DocumentID: strings.TrimSpace(documentID),
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func encodeSearchIndexSpacePayload(spaceID string) (string, error) {
	value, err := json.Marshal(searchIndexSpacePayload{
		SpaceID: strings.TrimSpace(spaceID),
	})
	if err != nil {
		return "", err
	}
	return string(value), nil
}
