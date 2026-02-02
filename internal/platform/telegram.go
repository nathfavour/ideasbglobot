package platform

import (
	"fmt"
	"log"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramPlatform struct {
	bot    *tgbotapi.BotAPI
	stopCh chan struct{}
}

func NewTelegramPlatform(token string) (*TelegramPlatform, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &TelegramPlatform{
		bot:    bot,
		stopCh: make(chan struct{}),
	}, nil
}

func (p *TelegramPlatform) Name() string {
	return "telegram"
}

func (p *TelegramPlatform) Listen(handler func(IncomingMessage) error) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := p.bot.GetUpdatesChan(u)

	log.Printf("Listening on Telegram as %s", p.bot.Self.UserName)

	for {
		select {
		case <-p.stopCh:
			return nil
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			username := update.Message.From.UserName
			if username == "" {
				username = update.Message.From.FirstName
			}

			msg := IncomingMessage{
				Platform:  "telegram",
				ChatID:    update.Message.Chat.ID,
				UserID:    update.Message.From.ID,
				Username:  username,
				Text:      update.Message.Text,
				IsBot:     update.Message.From.IsBot,
				IsCommand: update.Message.IsCommand(),
				Raw:       update,
			}

			if msg.IsCommand {
				msg.Command = update.Message.Command()
				msg.Args = update.Message.CommandArguments()
			}

			if err := handler(msg); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		}
	}
}

func (p *TelegramPlatform) Send(msg OutgoingMessage) error {
	reply := tgbotapi.NewMessage(msg.ChatID, msg.Text)
	_, err := p.bot.Send(reply)
	return err
}

func (p *TelegramPlatform) Stop() {
	close(p.stopCh)
}
