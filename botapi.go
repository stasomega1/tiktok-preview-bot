package main

import (
	"fmt"
	"github.com/caarlos0/env/v6"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TgBot struct {
	ApiKey string `env:"BOT_API_KEY,required"`
	BotApi *tgbotapi.BotAPI
}

func NewTgBot() (*TgBot, error) {
	tgBot := &TgBot{}
	err := tgBot.ParseEnvs()
	if err != nil {
		return nil, err
	}

	tgBot.BotApi, err = tgbotapi.NewBotAPI(tgBot.ApiKey)
	if err != nil {
		return nil, fmt.Errorf("NewBotApi error: %v", err)
	}

	return tgBot, nil
}

func (t *TgBot) ParseEnvs() error {
	err := env.Parse(t)
	if err != nil {
		return fmt.Errorf("env parsing error: %v", err)
	}
	return nil
}

func (t *TgBot) SetBot() (err error) {
	t.BotApi, err = tgbotapi.NewBotAPI(t.ApiKey)
	return
}

func (t *TgBot) GetUpdates() tgbotapi.UpdatesChannel {
	return t.BotApi.GetUpdatesChan(tgbotapi.UpdateConfig{
		Offset:         0,
		Limit:          1,
		Timeout:        1,
		AllowedUpdates: nil,
	})
}

func (t *TgBot) SendVideo(update tgbotapi.Update, videoUrl string) error {
	mediaVideo := tgbotapi.NewInputMediaVideo(tgbotapi.FileURL(videoUrl))
	mediaVideo.Caption = fmt.Sprintf("Sender: @%s\nOriginal message: %s",
		update.Message.From.UserName, update.Message.Text)
	mediaGroup := tgbotapi.NewMediaGroup(update.FromChat().ID, []interface{}{mediaVideo})
	mediaGroup.DisableNotification = true
	_, err := t.BotApi.SendMediaGroup(mediaGroup)
	return err
}
