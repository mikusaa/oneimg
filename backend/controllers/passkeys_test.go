package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestPasskeyCeremonyIsConsumedOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("passkey-test", cookie.NewStore([]byte("passkey-session-secret"))))
	router.POST("/begin", func(c *gin.Context) {
		err := savePasskeyCeremony(c, passkeyLoginSessionKey, passkeyCeremony{Session: webauthn.SessionData{
			Challenge: "latest-challenge",
			Expires:   time.Now().Add(time.Minute),
		}})
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.POST("/finish", func(c *gin.Context) {
		ceremony, err := consumePasskeyCeremony(c, passkeyLoginSessionKey)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if ceremony.Session.Challenge != "latest-challenge" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	begin := httptest.NewRecorder()
	router.ServeHTTP(begin, httptest.NewRequest(http.MethodPost, "/begin", nil))
	if begin.Code != http.StatusNoContent {
		t.Fatalf("begin status = %d", begin.Code)
	}
	cookies := begin.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("begin did not set a session cookie")
	}

	finishRequest := httptest.NewRequest(http.MethodPost, "/finish", nil)
	finishRequest.AddCookie(cookies[0])
	finish := httptest.NewRecorder()
	router.ServeHTTP(finish, finishRequest)
	if finish.Code != http.StatusNoContent {
		t.Fatalf("first finish status = %d", finish.Code)
	}
	consumedCookies := finish.Result().Cookies()
	if len(consumedCookies) == 0 {
		t.Fatal("finish did not persist consumed ceremony")
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/finish", nil)
	replayRequest.AddCookie(consumedCookies[0])
	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, replayRequest)
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want %d", replay.Code, http.StatusBadRequest)
	}
}
