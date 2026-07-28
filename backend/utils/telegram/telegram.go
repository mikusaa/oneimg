package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BotToken string
	Timeout  time.Duration
	Retry    int
}

type Message struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type Response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

type PlaceholderData struct {
	Username    string
	Date        string
	Filename    string
	StorageType string
	URL         string
}

func NewClient(botToken string) *Config {
	return &Config{
		BotToken: botToken,
		Timeout:  10 * time.Second,
		Retry:    2,
	}
}

func ReplacePlaceholders(template string, data PlaceholderData) string {
	result := template
	replacements := map[string]string{
		"username":    data.Username,
		"date":        data.Date,
		"filename":    data.Filename,
		"StorageType": data.StorageType,
		"url":         data.URL,
	}
	for key, value := range replacements {
		result = strings.ReplaceAll(result, "{"+key+"}", value)
	}
	return result
}

func (c *Config) SendMsg(msg Message, placeholderData PlaceholderData) error {
	if c.BotToken == "" {
		return fmt.Errorf("bot token 不能为空")
	}
	if msg.ChatID == "" {
		return fmt.Errorf("chat_id 不能为空")
	}

	messageText := msg.Text
	if messageText == "" {
		messageText = "{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]"
	}
	messageText += "\n\n访问链接:{url}"
	msg.Text = ReplacePlaceholders(messageText, placeholderData)
	if msg.Text == "" {
		return fmt.Errorf("替换占位符后消息文本为空")
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("消息序列化失败: %w", err)
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.BotToken)

	var lastErr error
	for i := 0; i <= c.Retry; i++ {
		lastErr = c.sendRequest(apiURL, msgBytes)
		if lastErr == nil {
			return nil
		}
		if i < c.Retry {
			time.Sleep(time.Duration(1<<i) * 500 * time.Millisecond)
		}
	}
	return fmt.Errorf("重试%d次后仍发送失败: %w", c.Retry, lastErr)
}

func (c *Config) sendRequest(apiURL string, msgBytes []byte) error {
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(msgBytes))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := (&http.Client{Timeout: c.Timeout}).Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	var telegramResponse Response
	if err := json.NewDecoder(resp.Body).Decode(&telegramResponse); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if !telegramResponse.OK {
		return fmt.Errorf("telegram API 错误 [code:%d]: %s", telegramResponse.ErrorCode, telegramResponse.Description)
	}
	return nil
}

func SendSimpleMsg(botToken, chatID, text string, placeholderData PlaceholderData) error {
	client := NewClient(botToken)
	return client.SendMsg(Message{
		ChatID: chatID,
		Text:   text,
	}, placeholderData)
}
