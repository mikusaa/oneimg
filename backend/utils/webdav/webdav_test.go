package webdav

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"oneimg/backend/utils/storageobject"
	"sync/atomic"
	"testing"
)

func TestPublicMediaReadOnlyAndNoRedirect(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	c := Client(Config{BaseURL: upstream.URL, Username: "private", Password: "secret"})
	for _, method := range []string{"DELETE", "PUT", "POST", "PATCH", "PROPFIND", "OPTIONS"} {
		if _, err := c.WebDAVMedia(context.Background(), "file", method, "", ""); err == nil {
			t.Fatal(method)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("write reached upstream")
	}
	for _, method := range []string{"GET", "HEAD"} {
		resp, err := c.WebDAVMedia(context.Background(), "file", method, "", "")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 307 {
			t.Fatal("redirect followed")
		}
	}
	if requests.Load() != 2 {
		t.Fatal("unexpected upstream count")
	}
}

func TestConditionalCreateAndEncodedHistoricalPaths(t *testing.T) {
	status := 201
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "*" || r.Method != "PUT" {
			t.Error("missing conditional create")
		}
		if r.URL.Path != `/name"#?%2F.png` || r.URL.RawQuery != "" {
			t.Error("path reinterpreted", r.URL)
		}
		w.WriteHeader(status)
	}))
	defer upstream.Close()
	c := Client(Config{BaseURL: upstream.URL})
	for _, test := range []struct {
		status int
		err    error
	}{{201, nil}, {412, storageobject.ErrConflict}, {204, storageobject.ErrConditionalUnsupported}} {
		status = test.status
		err := c.WebDAVCreate(context.Background(), `/name"#?%2F.png`, bytes.NewReader([]byte("data")))
		if !errors.Is(err, test.err) {
			t.Fatal(test.status, err)
		}
	}
}
