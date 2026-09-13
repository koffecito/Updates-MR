package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
)

func main() {
	mode := os.Getenv("MODE")
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	botToken := requireEnv("TG_BOT_TOKEN")
	appID := requireEnv("APP_ID")

	stateFile := os.Getenv("STATE_FILE")
	if stateFile == "" {
		stateFile = "state.json"
	}
	photoPath := os.Getenv("PHOTO_PATH")
	if photoPath == "" {
		photoPath = "assets/update_banner.png"
	}

	state := loadState(stateFile)

	switch mode {
	case "poll":
		ownerID, err := strconv.ParseInt(requireEnv("OWNER_USER_ID"), 10, 64)
		if err != nil {
			log.Fatalf("OWNER_USER_ID должен быть числом: %v", err)
		}
		runPoll(botToken, appID, ownerID, photoPath, &state)
	case "check":
		runCheck(botToken, appID, photoPath, &state)
	default:
		log.Fatal("использование: go run . [poll|check]")
	}

	if err := saveState(stateFile, state); err != nil {
		log.Fatalf("не удалось сохранить state.json: %v", err)
	}
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("не задана обязательная переменная окружения %s", name)
	}
	return v
}

func runPoll(botToken, appID string, ownerID int64, photoPath string, state *State) {
	updates, err := getUpdates(botToken, state.UpdateOffset)
	if err != nil {
		log.Fatalf("не удалось получить обновления: %v", err)
	}

	for _, u := range updates {
		state.UpdateOffset = u.UpdateID + 1

		if u.MyChatMember == nil {
			continue
		}
		cm := u.MyChatMember

		wasIn := isActiveMember(cm.OldChatMember.Status)
		nowIn := isActiveMember(cm.NewChatMember.Status)

		switch {
		case !wasIn && nowIn:
			if cm.From.ID != ownerID {
				log.Printf("чужое добавление в чат %d от пользователя %d — покидаю чат", cm.Chat.ID, cm.From.ID)
				_ = leaveChat(botToken, cm.Chat.ID)
				continue
			}

			threadID := 0
			if cm.Chat.IsForum {
				tid, err := createForumTopic(botToken, cm.Chat.ID, "🔔 Обновления приложений")
				if err != nil {
					log.Printf("не удалось создать тему в чате %d: %v", cm.Chat.ID, err)
					continue
				}
				threadID = tid
			}

			state.addChat(ConnectedChat{ChatID: cm.Chat.ID, ThreadID: threadID, Title: cm.Chat.Title})

			version, _, vErr := fetchPlayStoreInfo(appID)
			versionText := version
			if vErr != nil {
				versionText = "не удалось получить (" + vErr.Error() + ")"
			}

			caption := fmt.Sprintf(
				"✅ Тема подключена!\nБуду присылать сюда уведомления об обновлениях приложения.\n\nТекущая версия: %s",
				versionText,
			)
			if err := sendUpdatePhoto(botToken, state, cm.Chat.ID, threadID, caption, photoPath, "", ""); err != nil {
				log.Printf("не удалось отправить подтверждение в чат %d: %v", cm.Chat.ID, err)
			}

		case wasIn && !nowIn:
			state.removeChat(cm.Chat.ID)
		}
	}
}

func runCheck(botToken, appID, photoPath string, state *State) {
	version, whatsNew, err := fetchPlayStoreInfo(appID)
	if err != nil {
		log.Fatalf("ошибка получения данных из Google Play: %v", err)
	}

	if version == state.AppVersion {
		fmt.Println("версия не изменилась:", version)
		return
	}

	caption := fmt.Sprintf(
		"📦 Новое обновление!\n\nПриложение: %s\nВерсия: %s\n\nЧто нового:\n%s",
		appID, version, whatsNew,
	)
	buttonURL := "https://play.google.com/store/apps/details?id=" + url.QueryEscape(appID)

	for _, c := range state.Chats {
		if err := sendUpdatePhoto(botToken, state, c.ChatID, c.ThreadID, caption, photoPath, "✅ Обновить!", buttonURL); err != nil {
			log.Printf("не удалось отправить в чат %d (%s): %v", c.ChatID, c.Title, err)
		}
	}

	state.AppVersion = version
	fmt.Println("отправлено уведомление, версия обновлена на", version)
}

func isActiveMember(status string) bool {
	return status == "member" || status == "administrator" || status == "creator"
}

func sendUpdatePhoto(botToken string, state *State, chatID int64, threadID int, caption, photoPath, buttonText, buttonURL string) error {
	if state.PhotoFileID == "" {
		fileID, err := sendPhotoFromFile(botToken, chatID, threadID, photoPath, caption, buttonText, buttonURL)
		if err != nil {
			return err
		}
		state.PhotoFileID = fileID
		return nil
	}
	return sendPhotoByFileID(botToken, chatID, threadID, state.PhotoFileID, caption, buttonText, buttonURL)
}
