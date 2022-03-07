package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"time"
)

func main() {
	api, err := RunTelegramApi()
	if err != nil {
		log.Panic(err)
	}
	atmStateChan := getAtmStateChan(time.Second * 60)
	application, err := NewApplication(api, atmStateChan)
	if err != nil {
		log.Panic(err)
	}
	application.BeginAtmStateProcessing()
}

type SubscriptionConfig struct {
	MinLimits map[Currency]Limit
}

type Subscribers map[int64]SubscriptionConfig

func (s *SubscriptionConfig) Accepts(limit *Limit) bool {
	if len(s.MinLimits) == 0 { //no configuration == accept everything
		return true
	}
	configLimit, ok := s.MinLimits[limit.Currency]
	return ok && configLimit.Amount >= limit.Amount
}

type Application struct {
	api         *tgbotapi.BotAPI
	stateChan   chan *State
	subscribers Subscribers
}

func NewApplication(api *tgbotapi.BotAPI, atmStateChan chan *State) (Application, error) {
	var subscribers Subscribers = make(map[int64]SubscriptionConfig)

	actionChan := make(chan UserAction)
	go beginTgPollingLoop(api, actionChan)
	go func() {
		for {
			action := <-actionChan
			if action.Type == Subscribe {
				subscribers[action.ChatId] = SubscriptionConfig{}
			} else if action.Type == Unsubscribe {
				delete(subscribers, action.ChatId)
			}
		}

	}()
	return Application{api: api, stateChan: atmStateChan, subscribers: subscribers}, nil
}

func (app *Application) BeginAtmStateProcessing() {
	prevPoints := make(map[string]int)
	currentStamp := 0
	for state := range app.stateChan {
		currentStamp++
		for _, clusters := range state.AtmStatePayload.Clusters {
			for _, point := range clusters.Points {
				if _, ok := prevPoints[point.Id]; !ok {
					app.sendPointToUsers(&point)
				}
				prevPoints[point.Id] = currentStamp
			}
		}
		for prev := range prevPoints {
			if currentStamp-prevPoints[prev] > 5 {
				delete(prevPoints, prev)
			}
		}
	}
}

func (app Application) sendPointToUsers(point *Point) {
	msgText := fmt.Sprintf("New ATM💰 \nAddress: %v \nAvailable: %v", point.Address, point.Limits)
	log.Printf(msgText)

	for chatId, config := range app.subscribers {
		if !accepts(point, config) {
			continue
		}
		msg := tgbotapi.NewMessage(chatId, msgText)
		_, err := app.api.Send(msg)
		if err != nil {
			log.Printf("error while sending msg to %v %v", chatId, err)
		}
	}
}

func accepts(point *Point, config SubscriptionConfig) bool {
	shouldSend := false
	for _, limit := range point.Limits {
		if config.Accepts(&limit) {
			shouldSend = true
		}
	}
	return shouldSend
}
