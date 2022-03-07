package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type State struct {
	AtmStatePayload StatePayload `json:"payload"`
}
type StatePayload struct {
	Clusters []Cluster `json:"clusters"`
}

type Cluster struct {
	Points []Point `json:"points"`
}

type Point struct {
	Id      string  `json:"id"`
	Address string  `json:"address"`
	Limits  []Limit `json:"limits"`
}
type Limit struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type Request struct {
	Bounds  Bounds  `json:"bounds"`
	Filters Filters `json:"filters"`
	Zoom    int     `json:"zoom"`
}

type Bounds struct {
	BottomLeft GeoPoint `json:"bottomLeft"`
	TopRight   GeoPoint `json:"topRight"`
}

type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Filters struct {
	Banks           []string `json:"banks"`
	ShowUnavailable bool     `json:"showUnavailable"`
	Currencies      []string `json:"currencies"`
}

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

				users.Store(action.ChatId, action.UserName)
				userActionChan <- action
			}

			msg := tgbotapi.NewMessage(update.Message.From.ID, replyText)
			msg.ReplyToMessageID = update.Message.MessageID
			_, _ = bot.Send(msg)
		}
	}
}

var users = sync.Map{}

func sendPointToUsers(point *Point, api *tgbotapi.BotAPI) {
	users.Range(func(k interface{}, v interface{}) bool {
		chatId := k.(int64)
		msgText := fmt.Sprintf("New atm \nAddress: %v \nAvailable: %v", point.Address, point.Limits)
		log.Printf(msgText)
		msg := tgbotapi.NewMessage(chatId, msgText)
		_, err := api.Send(msg)
		if err != nil {
			log.Printf("error while sending msg to %v %v", chatId, err)
		}
		return true
	})
}

func main() {
	api, err := RunTelegramApi()
	if err != nil {
		log.Panic(err)
	}
	go beginTgPollingLoop(api, make(chan UserAction))
	atmStateChan := getAtmStateChan(time.Second * 60)
	beginAtmStateProcessing(api, atmStateChan)
}

func beginAtmStateProcessing(api *tgbotapi.BotAPI, stateChan chan *State) {
	prevPoints := make(map[string]int)
	currentStamp := 0
	for state := range stateChan {
		currentStamp++
		for _, clusters := range state.AtmStatePayload.Clusters {
			for _, point := range clusters.Points {
				if _, ok := prevPoints[point.Id]; !ok {
					sendPointToUsers(&point, api)
				}
				prevPoints[point.Id] = currentStamp
			}
		}
		for prev, _ := range prevPoints {
			if currentStamp-prevPoints[prev] > 5 {
				delete(prevPoints, prev)
			}
		}
	}
}

func readUpdate(request *Request) (state *State, err error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		log.Printf("can't marshall %v", request)
		return
	}

	resp, err := http.DefaultClient.Post("https://api.tinkoff.ru/geo/withdraw/clusters", "application/json", bytes.NewReader(reqBody))

	if err != nil {
		log.Printf("Error while fetching atm states")
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	state = &State{}
	err = json.Unmarshal(body, state)
	return
}

func getAtmStateChan(interval time.Duration) chan *State {
	var atmStateChan = make(chan *State)
	go func() {
		for {
			state, err := readUpdate(&Request{
				Bounds: Bounds{
					BottomLeft: GeoPoint{Lat: 59.786322383354694, Lng: 30.057925550468347},
					TopRight:   GeoPoint{Lat: 60.01553006637703, Lng: 30.56055006218709},
				},
				Filters: Filters{
					Banks:           []string{"tcs"},
					ShowUnavailable: true,
					Currencies:      []string{"USD"},
				},
				Zoom: 12,
			})
			if err != nil {
				return
			}

			log.Printf("received %v", state)

			atmStateChan <- state
			time.Sleep(interval)
		}
	}()
	return atmStateChan
}
