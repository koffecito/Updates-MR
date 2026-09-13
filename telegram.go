package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

const tgAPI = "https://api.telegram.org/bot"

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

type ChatInfo struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	IsForum bool   `json:"is_forum"`
}

type ChatMemberInfo struct {
	Status string `json:"status"`
}

type FromUser struct {
	ID int64 `json:"id"`
}

type ChatMemberUpdated struct {
	Chat          ChatInfo       `json:"chat"`
	From          FromUser       `json:"from"`
	OldChatMember ChatMemberInfo `json:"old_chat_member"`
	NewChatMember ChatMemberInfo `json:"new_chat_member"`
}

type Update struct {
	UpdateID     int64              `json:"update_id"`
	MyChatMember *ChatMemberUpdated `json:"my_chat_member,omitempty"`
}

func callJSON(botToken, method string, payload map[string]interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(tgAPI+botToken+"/"+method, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseTgResponse(resp)
}

func parseTgResponse(resp *http.Response) (json.RawMessage, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("не удалось распарсить ответ Telegram: %w (тело: %s)", err, string(raw))
	}
	if !r.OK {
		return nil, fmt.Errorf("telegram api ошибка: %s", r.Description)
	}
	return r.Result, nil
}

func getUpdates(botToken string, offset int64) ([]Update, error) {
	payload := map[string]interface{}{
		"offset":          offset,
		"timeout":         0,
		"allowed_updates": []string{"my_chat_member"},
	}
	result, err := callJSON(botToken, "getUpdates", payload)
	if err != nil {
		return nil, err
	}
	var updates []Update
	if err := json.Unmarshal(result, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func createForumTopic(botToken string, chatID int64, name string) (int, error) {
	result, err := callJSON(botToken, "createForumTopic", map[string]interface{}{
		"chat_id": chatID,
		"name":    name,
	})
	if err != nil {
		return 0, err
	}
	var out struct {
		MessageThreadID int `json:"message_thread_id"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, err
	}
	return out.MessageThreadID, nil
}

func leaveChat(botToken string, chatID int64) error {
	_, err := callJSON(botToken, "leaveChat", map[string]interface{}{"chat_id": chatID})
	return err
}

func sendTextMessage(botToken string, chatID int64, threadID int, text string) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	if threadID != 0 {
		payload["message_thread_id"] = threadID
	}
	_, err := callJSON(botToken, "sendMessage", payload)
	return err
}

func sendPhotoByFileID(botToken string, chatID int64, threadID int, fileID, caption string, buttonText, buttonURL string) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"photo":      fileID,
		"caption":    caption,
		"parse_mode": "HTML",
	}
	if threadID != 0 {
		payload["message_thread_id"] = threadID
	}
	if buttonText != "" && buttonURL != "" {
		payload["reply_markup"] = map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{{"text": buttonText, "url": buttonURL}},
			},
		}
	}
	_, err := callJSON(botToken, "sendPhoto", payload)
	return err
}

func sendPhotoFromFile(botToken string, chatID int64, threadID int, filePath, caption string, buttonText, buttonURL string) (fileID string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	_ = w.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
	if threadID != 0 {
		_ = w.WriteField("message_thread_id", fmt.Sprintf("%d", threadID))
	}
	if buttonText != "" && buttonURL != "" {
		markup, _ := json.Marshal(map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{{"text": buttonText, "url": buttonURL}},
			},
		})
		_ = w.WriteField("reply_markup", string(markup))
	}

	part, err := w.CreateFormFile("photo", "update_banner.png")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", tgAPI+botToken+"/sendPhoto", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	result, err := parseTgResponse(resp)
	if err != nil {
		return "", err
	}

	var out struct {
		Photo []struct {
			FileID string `json:"file_id"`
		} `json:"photo"`
	}
	if err := json.Unmarshal(result, &out); err != nil {
		return "", err
	}
	if len(out.Photo) == 0 {
		return "", fmt.Errorf("telegram не вернул file_id фото")
	}
	return out.Photo[len(out.Photo)-1].FileID, nil
}
