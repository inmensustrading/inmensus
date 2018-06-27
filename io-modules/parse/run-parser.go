package main

import (
  "fmt"
  gdax "github.com/preichenberger/go-gdax"
  ws "github.com/gorilla/websocket"
)

//this will 
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

		if message.Type == "snapshot" {
			wsConn.ReadJSON(&message)
			productId := message.ProductId
			asks := message.Asks
			bids := message.Bids
			outerAskLength := len(asks)
			innerAskLength := len(asks[0])
			fmt.Printf("productId: %s", productId)
			for i := 0; i < outerAskLength; i++ {
				for j := 0; j < innerAskLength; j++ {
					fmt.Printf("Ask: %s \n",asks[i][j])
					//fmt.Printf("i,j: %d%d \n", i, j)
				}
			}
			outerBidLength := len(bids)
			innerBidLength := len(bids[0])
			for i := 0; i < outerBidLength; i++ {
				for j := 0; j < innerBidLength; j++ {
				 	fmt.Printf("Bid: %s \n", bids[i][j])
				 }
				
			}
		}
	}
}




