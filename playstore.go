package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

func fetchPlayStoreInfo(appID string) (version string, whatsNew string, err error) {
	pageURL := "https://play.google.com/store/apps/details?id=" + url.QueryEscape(appID) + "&hl=ru&gl=RU"

	req, _ := http.NewRequest("GET", pageURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	html := string(body)

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("неожиданный статус %d (возможно, неверный APP_ID или капча/бан по IP)", resp.StatusCode)
	}

	re := regexp.MustCompile(`AF_initDataCallback\({key: 'ds:5'.*?data:(\[.*?\]), sideChannel`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return "", "", fmt.Errorf("не удалось найти блок ds:5 на странице — вёрстка Google Play могла измениться")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return "", "", fmt.Errorf("не удалось распарсить JSON ds:5: %w", err)
	}

	version, vErr := digString(data, 1, 2, 140, 0, 0, 0)
	whats, wErr := digString(data, 1, 2, 144, 1, 1)

	if vErr != nil {
		return "", "", fmt.Errorf("не удалось извлечь версию по ожидаемым индексам (структура ds:5 могла измениться): %w", vErr)
	}
	if wErr != nil {
		whats = "(текст «что нового» не найден или отсутствует у этого приложения)"
	}

	return version, whats, nil
}

func digString(v interface{}, path ...int) (string, error) {
	cur := v
	for _, idx := range path {
		arr, ok := cur.([]interface{})
		if !ok {
			return "", fmt.Errorf("ожидался массив на шаге %d, получено %T", idx, cur)
		}
		if idx < 0 || idx >= len(arr) {
			return "", fmt.Errorf("индекс %d вне диапазона (длина %d)", idx, len(arr))
		}
		cur = arr[idx]
	}
	s, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("ожидалась строка, получено %T", cur)
	}
	return s, nil
}
