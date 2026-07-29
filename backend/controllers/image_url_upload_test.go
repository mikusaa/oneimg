package controllers

import (
	"context"
	"encoding/json"
	"testing"
)

func TestURLUploadRequestAcceptsNumericAndStringIDs(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantTag    int
		wantBucket int
	}{
		{name: "numeric", payload: `{"url":"https://example.com/a.png","tag_id":2,"bucket_id":3}`, wantTag: 2, wantBucket: 3},
		{name: "string", payload: `{"url":"https://example.com/a.png","tag_id":"2","bucket_id":"3"}`, wantTag: 2, wantBucket: 3},
		{name: "empty", payload: `{"url":"https://example.com/a.png","tag_id":"","bucket_id":null}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request urlUploadRequest
			if err := json.Unmarshal([]byte(tt.payload), &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if int(request.TagID) != tt.wantTag || int(request.BucketID) != tt.wantBucket {
				t.Fatalf("ids = %d/%d, want %d/%d", request.TagID, request.BucketID, tt.wantTag, tt.wantBucket)
			}
		})
	}
}

func TestNewRemoteImageRequestSetsPixivReferer(t *testing.T) {
	request, err := newRemoteImageRequest(context.Background(), "https://i.pximg.net/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Referer"); got != "https://www.pixiv.net/" {
		t.Fatalf("Referer = %q", got)
	}

	request, err = newRemoteImageRequest(context.Background(), "https://example.com/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if got := request.Header.Get("Referer"); got != "" {
		t.Fatalf("unexpected Referer = %q", got)
	}
}
