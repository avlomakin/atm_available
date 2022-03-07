package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Currency string

const (
	USD Currency = "USD"
	RUB Currency = "RUB"
	EUR Currency = "EUR"
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
	Currency Currency `json:"currency"`
	Amount   int64    `json:"amount"`
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

	//goland:noinspection GoUnhandledErrorResult
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
