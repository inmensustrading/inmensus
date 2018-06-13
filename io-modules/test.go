package io

import (
  "fmt"
  gdax "github.com/preichenberger/go-gdax"
  ws "github.com/gorilla/websocket"
)

func main() {
	var wsDialer ws.Dialer
	wsConn, _, err := wsDialer.Dial("wss://ws-feed.gdax.com", nil)
	if err != nil {
		println(err.Error())
	}
	
	
	subscribe := gdax.Message{
		Type:      "subscribe",
		Channels: []gdax.MessageChannel{
		gdax.MessageChannel{
			Name: "level2",
			ProductIds: []string{
			"BTC-USD",
			},
		},
		},
	}
	if err := wsConn.WriteJSON(subscribe); err != nil {
		println(err.Error())
	}
	
	message:= gdax.Message{}
	for true {
		if err := wsConn.ReadJSON(&message); err != nil {
			println(err.Error())
			break
		}

		if message.Type == "l2update" {
			wsConn.ReadJSON(&message)
			changes := message.Changes
			outerLength := len(changes)
			innerLength := len(changes[0])
			for i := 0; i < outerLength; i++ {
				for j := 0; j < innerLength; j++ {
					fmt.Printf("%s \n",changes[i][j])
					fmt.Printf("  i,j: %d%d \n", i, j)
				}
			}
		}
		
		if message.Type == "match" {
			println("Got a match")
		}
		
	}
}




