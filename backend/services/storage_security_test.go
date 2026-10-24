package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
)

func TestStorageTestAuthorizationAndCredentialBoundary(t *testing.T) {
	db := newServiceTestDB(t)
	config.App = &config.Config{ConfigSecret: "test-secret"}
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()
	service := NewStorageService(db)
	if err := db.Create(&models.Buckets{Id: 1, Name: "default", Type: "default"}).Error; err != nil {
		t.Fatal(err)
	}
	input := StorageInput{Name: "test", Type: "webdav", Config: map[string]any{"webdav_url": upstream.URL, "webdav_user": "user", "webdav_pass": "secret"}}
	if err := database.SyncSequences(db, nil); err != nil {
		t.Fatal(err)
	}
	bucket, err := service.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	actor := models.User{ID: 2, Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"storage:create"}}}
	for _, user := range []models.User{actor, {ID: 3, Role: models.RoleUser}} {
		_, err = service.TestConnection(context.Background(), user, input, &bucket.Id)
		if !errors.Is(err, ErrPermissionDenied) {
			t.Fatal(err)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("unauthorized network request")
	}
	if _, err = service.TestConnection(context.Background(), actor, input, nil); err != nil {
		t.Fatal(err)
	}
	actor.Permission.Codes = []string{"storage:update"}
	if _, err = service.TestConnection(context.Background(), actor, StorageInput{}, &bucket.Id); err != nil {
		t.Fatal(err)
	}
	before := requests.Load()
	for _, change := range []map[string]any{{"webdav_url": upstream.URL + "/other"}, {"webdav_user": "other"}} {
		if _, err = service.TestConnection(context.Background(), actor, StorageInput{Config: change}, &bucket.Id); !errors.Is(err, ErrNewEndpointCredentials) {
			t.Fatal(err)
		}
		if _, err = service.Update(bucket.Id, StorageInput{Name: "new", Type: "webdav", Config: change}); !errors.Is(err, ErrNewEndpointCredentials) {
			t.Fatal(err)
		}
	}
	if _, err = service.TestConnection(context.Background(), actor, StorageInput{Type: "ftp"}, &bucket.Id); !errors.Is(err, ErrStorageTypeInvalid) {
		t.Fatal(err)
	}
	if requests.Load() != before {
		t.Fatal("invalid destination reached network")
	}
}
