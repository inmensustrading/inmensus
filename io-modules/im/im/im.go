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
	"strings"

	"../../iombase"
	"github.com/inmensustrading/inmensus/strategies/strategybase"
	gdax "github.com/preichenberger/go-gdax"
)

//list of all strategies connected
var stratList []*rpc.Client

//InputModuleServer needs this declaration in every IM
type InputModuleServer int

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

	subscribe := gdax.Message{
		Type: "subscribe",
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

	message := gdax.Message{}

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {

		/*
			fmt.Print("Enter command: ")
			text, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal("error reading command:", err)
			}
			text = strings.TrimSpace(text)
		*/
		//Start the websocket feed right now
		if err := wsConn.ReadJSON(&message); err != nil {
			println(err.Error())
			break
		}

		//do if-statement based on type of event
		/*
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
			} else {
				fmt.Println("Unrecognized command.")
			}
		*/

		//my understanding is that snapshots only occur after you subscribe to the
		//level2 channel and all other events that come pretty much are l2updates
		if message.Type == "snapshot" {
			fmt.Println("sending initial l2 snapshot to strats")
			//Access the message data once instead of every time through loop
			currencyType := message.ProductId
			askData := make([]iombase.Ask)
			for i := 0; i < len(message.Asks); i++ {
				ask := &iombase.Ask{
					price: message.Asks[i][0],
					size:  message.Asks[i][1],
				}
				askData.append(ask)
			}

			randomAsk := askData[0]

			for a := 0; a < len(stratList); a++ {
				args := &strategybase.OnInputEventArgs{
					ExchangeName: "gdax",
					EventType:    iombase.L2SnapshotAsk,
					Currency:     currencyType,
					Volume:       randomAsk.size,
					Prce:         randomAsk.price,
				}
				var reply int
				err = stratList[a].Call("StrategyServer.OnInputEvent", args, &reply)
				if err != nil {
					fmt.Println(err)
				}
			}

		} else {
			fmt.Println("a non L2 snapshot event is happening with type: " + message.Type)
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
