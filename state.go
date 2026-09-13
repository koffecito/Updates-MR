package main

import (
	"encoding/json"
	"os"
)

type ConnectedChat struct {
	ChatID   int64  `json:"chat_id"`
	ThreadID int    `json:"thread_id,omitempty"`
	Title    string `json:"title"`
}

type State struct {
	AppVersion   string          `json:"app_version"`
	UpdateOffset int64           `json:"update_offset"`
	PhotoFileID  string          `json:"photo_file_id,omitempty"`
	Chats        []ConnectedChat `json:"chats"`
}

func loadState(path string) State {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var s State
	_ = json.Unmarshal(data, &s)
	return s
}

func saveState(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *State) hasChat(chatID int64) bool {
	for _, c := range s.Chats {
		if c.ChatID == chatID {
			return true
		}
	}
	return false
}

func (s *State) addChat(c ConnectedChat) {
	if s.hasChat(c.ChatID) {
		return
	}
	s.Chats = append(s.Chats, c)
}

func (s *State) removeChat(chatID int64) {
	out := s.Chats[:0]
	for _, c := range s.Chats {
		if c.ChatID != chatID {
			out = append(out, c)
		}
	}
	s.Chats = out
}
