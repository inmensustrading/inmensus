package dummyim

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

	"../../../strategies/strategybase"
	"../../iombase"
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
func DummyIM(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Dummy IM.")

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

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for true {
		fmt.Print("Enter command: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("error reading command:", err)
		}
		text = strings.TrimSpace(text)

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
	}

	//clean up and conclude
	fmt.Println("Exiting...")
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}
