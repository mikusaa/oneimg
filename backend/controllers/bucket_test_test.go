package controllers

import "testing"

func TestBucketConnectionRejectsRemovedTelegramStorage(t *testing.T) {
	_, err := buildBucketConnectionCandidate(map[string]any{
		"type":         "telegram",
		"tg_bot_token": "token",
		"tg_receivers": "chat",
	})
	if err == nil {
		t.Fatal("buildBucketConnectionCandidate() should reject removed Telegram storage")
	}
}
