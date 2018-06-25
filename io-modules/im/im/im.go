package im

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"

	ws "github.com/gorilla/websocket"
	iombase "github.com/inmensustrading/inmensus/io-modules/iombase"
	"github.com/inmensustrading/inmensus/strategies/strategybase"
	gdax "github.com/preichenberger/go-gdax"
)

//InputModuleServer needs this declaration in every IM
type InputModuleServer int

//list of all strategies connected
var stratList []*rpc.Client

//RegisterStrategy read the docs
func (t *InputModuleServer) RegisterStrategy(args *iombase.RegisterStrategyArgs, reply *int) error {
	fmt.Println("Strategy registered at port " + (*args).StrategyPort + " named '" + (*args).StrategyName + "'.")

	client, err := rpc.DialHTTP("tcp", "localhost:"+(*args).StrategyPort)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Connected to input module at port " + (*args).StrategyPort + ".")

		stratList = append(stratList, client)
	}
	return nil
}

//Subscribe to a gdax channel and return the results
//pass in pointer to ws conn, and the name of a channel, and subscribe to it
func subscribeToChannel(websock *ws.Conn, msg *gdax.Message, channelName string) {
	subscribe := gdax.Message{
		Type: "subscribe",
		Channels: []gdax.MessageChannel{
			gdax.MessageChannel{
				Name: channelName,
				ProductIds: []string{
					"BTC-USD",
				},
			},
		},
	}
	if err := websock.WriteJSON(subscribe); err != nil {
		println(err.Error())
	}

	if err := websock.ReadJSON(&msg); err != nil {
		println(err.Error())
	}

	//If we got to this point, then there were no errors
	fmt.Println("The message type is: " + msg.Type)
}

//Helper function to format Ask objects into OnInputEventArgs format
func formatAsks(asks []iombase.Ask, currency string) []*strategybase.OnInputEventArgs {
	var result []*strategybase.OnInputEventArgs

	for a := 0; a < len(asks); a++ {
		//turn each ask into a strat event
		askEvent := &strategybase.OnInputEventArgs{
			ExchangeName: "gdax",
			EventType:    iombase.L2SnapshotAsk,
			Currency:     currency,
			Volume:       asks[a].Size,
			Price:        asks[a].Price,
		}
		result = append(result, askEvent)
	}
	return result
}

//DummyIM external calling designation
func IM(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Real IM.")

	//read config into map
	dat, err := ioutil.ReadFile(configPath)
	checkError(err)
	configStr := string(dat)

	config := make(map[string]string)
	key, acc := "", ""
	for a := 0; a < len(configStr); a++ {
		if configStr[a] == ':' {
			key = strings.TrimSpace(acc)
			acc = ""
		} else if configStr[a] == '\n' && key != "" {
			config[key] = strings.TrimSpace(acc)
			acc = ""
			key = ""
		} else {
			acc += string(configStr[a])
		}
	}
	if key != "" {
		config[key] = strings.TrimSpace(acc)
	}

	fmt.Println("Configuration options:")
	fmt.Println(config)

	//setup listening
	server := new(InputModuleServer)
	rpc.Register(server)
	rpc.HandleHTTP()
	listen, err := net.Listen("tcp", ":"+config["port"])
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	go http.Serve(listen, nil)
	fmt.Println("Listening setup on port " + config["port"] + ".")

	//GDAX Websocket feed setup
	var wsDialer ws.Dialer
	wsConn, _, err := wsDialer.Dial("wss://ws-feed.gdax.com", nil)
	if err != nil {
		println(err.Error())
	}
	//This is the object we will use to store ws results
	message := gdax.Message{}

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {
		fmt.Print("Enter command: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("error reading command:", err)
		}
		//text = strings.TrimSpace(text)
		words := strings.Fields(text)
		text = words[0]
		channel := words[1]

		if text == "exit" {
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else if text == "test-event" {
			fmt.Println("Sending event to all connected strategies who have registered.")
			for a := 0; a < len(stratList); a++ {
				args := &strategybase.OnInputEventArgs{
					ExchangeName: "dummy-exchange",
					EventType:    iombase.PlaceBuy,
					Currency:     "BTC->ETH",
					Volume:       420.69,
				}
				var reply int
				err = stratList[a].Call("StrategyServer.OnInputEvent", args, &reply)
				if err != nil {
					fmt.Println(err)
				}
			}
		} else if text == "count" {
			fmt.Println("Strategies registered: ", len(stratList), ".")
		} else if text == "subscribe" {
			fmt.Println("Subscribing to channel: ", channel)
			subscribeToChannel(wsConn, &message, channel)
		} else {
			fmt.Println("Unrecognized command.")
		}

		//check the type of message
		//my understanding is that snapshots only occur after you subscribe to the
		//level2 channel and all other events that come pretty much are l2updates
		if message.Type == "snapshot" {
			fmt.Println("sending initial l2 snapshot to strats")
			//Access the message data once instead of every time through loop
			currencyType := message.ProductId
			askData := make([]iombase.Ask, 10)
			for i := 0; i < len(message.Asks); i++ {
				sizeValue, errSize := strconv.ParseFloat(message.Asks[i][0], 64)
				priceValue, errPrice := strconv.ParseFloat(message.Asks[i][1], 64)
				if (errSize == nil) && (errPrice == nil) {
					ask := iombase.Ask{
						Price: priceValue,
						Size:  sizeValue,
					}
					askData = append(askData, ask)
				}
			}

			// have the array of Ask objects, need to turn them into OnInputEvent structs
			formattedAsks := formatAsks(askData, currencyType)

			for a := 0; a < len(stratList); a++ {
				var reply int
				for b := 0; b < len(formattedAsks); b++ {
					err = stratList[a].Call("StrategyServer.OnInputEvent", formattedAsks[b], &reply)
					if err != nil {
						fmt.Println(err)
					}
				}

			}
		}
	}
	//clean up and conclude
	fmt.Println("Exiting...")
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}
