package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func SendTelegramMessage(ctx context.Context, botToken, chatID, text string) error {
	if botToken == "" || chatID == "" || strings.TrimSpace(text) == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram status=%d", resp.StatusCode)
	}
	return nil
}
