package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"
)

//StrategyServer int-equivalent error code return type
type StrategyServer int

//OnInputEvent called by IOM when input event has arrived
func (t *StrategyServer) OnInputEvent(args *map[string]string, reply *int) error {
	(*args)["test"] = "success"
	return nil
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	//init
	fmt.Println("Starting...")
	fmt.Println("Orderbook pressure strategy, taken from https://goo.gl/HH6P7V.")

	//read config into map
	dat, err := ioutil.ReadFile("config.ini")
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
	l, e := net.Listen("tcp", ":"+config["strategy-port"])
	if e != nil {
		log.Fatal("Listen error:", e)
	}
	go http.Serve(l, nil)
	fmt.Println("Listening setup.")

	//connect to output modules
	if config["output-std"] != "yes" {
		omServers := []
		if config["use-gdax-iom"] == "yes"
			omServers.push(59998)

		client, err := rpc.DialHTTP("tcp", "localhost:"+)
		if err != nil {
			log.Fatal("Error while connecting to output modules:", err)
		}
	}

	//clean up and conclude
	fmt.Println("Exiting...")
}
