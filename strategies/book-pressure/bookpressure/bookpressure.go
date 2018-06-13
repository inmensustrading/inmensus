package bookpressure

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"strings"
)

//StrategyServer int-equivalent error code return type
type StrategyServer int

//OnInputEventArgs argument type for OnInputEvent
type OnInputEventArgs struct {
	exchangeName string
	eventType    string
	currency     string
	volume       float64
}

//OnInputEvent called by IOM when input event has arrived
func (t *StrategyServer) OnInputEvent(args *OnInputEventArgs, reply *int) error {
	(*args).exchangeName = "what"
	return nil
}

//BookPressure external calling designation
func BookPressure(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Orderbook pressure strategy, taken from https://goo.gl/HH6P7V.")

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
	stratServer := new(StrategyServer)
	rpc.Register(stratServer)
	rpc.HandleHTTP()
	listen, err := net.Listen("tcp", ":"+config["strategy-port"])
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	go http.Serve(listen, nil)
	fmt.Println("Listening setup on port " + config["strategy-port"] + ".")

	//fetch the ioms from the config
	iomHighPort, err := strconv.Atoi(config["iom-port-high"])
	if err != nil {
		log.Fatal("iom-port-high configuration:", err)
	}

	//breakdown the iom list
	ioms := config["iom-list"]
	iomList := make([]int, 0)
	acc = ""
	for a := 0; a < len(ioms); a++ {
		if ioms[a] != ' ' {
			acc += string(ioms[a])
		}
		if ioms[a] == ' ' || a == len(ioms)-1 {
			num, err := strconv.Atoi(acc)
			if err != nil {
				log.Fatal("iom-list configuration:", err)
			}
			iomList = append(iomList, num)
			acc = ""
		}
	}

	//connect to any output modules
	outModules := make([]*rpc.Client, len(iomList))
	if config["output-std"] != "yes" {
		//connect to the servers on the list
		for a := 0; a < len(iomList); a++ {
			port := strconv.Itoa(iomHighPort - iomList[a]*2)

			//TODO: move client to a higher scope
			client, err := rpc.DialHTTP("tcp", "localhost:"+port)
			if err != nil {
				log.Fatal("dialing:", err)
			}
			fmt.Println("Connected to port " + port)

			outModules[a] = client
		}
	}

	//set up command loop

	//clean up and conclude
	fmt.Println("Exiting...")
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}
