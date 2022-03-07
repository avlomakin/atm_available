package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
)

func RunTelegramApi() (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("ATM_BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)
	return bot, nil
}

type UserActionType int8

const (
	Subscribe UserActionType = iota
	Unsubscribe
)

type UserAction struct {
	ChatId   int64
	UserName string
	Type     UserActionType
}

func beginTgPollingLoop(bot *tgbotapi.BotAPI, userActionChan chan UserAction) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // If we got a message
			command := update.Message.Command()
			var replyText string
			if command == "" {
				replyText = "Unresolved command("
			} else if command == "start" {
				replyText = "Added to the mailing list"
				log.Printf("new recepient: %v, [%v]", update.Message.From.UserName, update.Message.Chat.ID)

				action := UserAction{
					Type:     Subscribe,
					ChatId:   update.Message.Chat.ID,
					UserName: update.Message.From.UserName,
				}
				userActionChan <- action
			} else if command == "end" {
				replyText = "Removed from the mailing list"
				log.Printf("removing recepient: %v, [%v]", update.Message.From.UserName, update.Message.Chat.ID)

				action := UserAction{
					Type:     Unsubscribe,
					ChatId:   update.Message.Chat.ID,
					UserName: update.Message.From.UserName,
				}
				userActionChan <- action
			}

			msg := tgbotapi.NewMessage(update.Message.From.ID, replyText)
			msg.ReplyToMessageID = update.Message.MessageID
			_, _ = bot.Send(msg)
		}
	}
}
