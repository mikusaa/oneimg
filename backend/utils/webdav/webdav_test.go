package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
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
