package dummystrategy

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
	"time"

	"github.com/inmensustrading/inmensus/io-modules/iombase"
	"github.com/inmensustrading/inmensus/strategies/strategybase"
)

//StrategyServer int-equivalent error code return type
type StrategyServer int

//OnInputEvent called by IOM when input event has arrived
func (t *StrategyServer) OnInputEvent(args *strategybase.OnInputEventArgs, reply *int) error {
	if (*args).EventType == iombase.PlaceBuy {
		fmt.Println("PlaceBuy received: exchange=" + (*args).ExchangeName + "; currency=" + (*args).Currency + "; volume=" + floatToString((*args).Volume))
	} else if (*args).EventType == iombase.PlaceSell {
		fmt.Println("PlaceSell received: exchange=" + (*args).ExchangeName + "; currency=" + (*args).Currency + "; volume=" + floatToString((*args).Volume))
	} else if (*args).EventType == iombase.RemoveBuy {
		fmt.Println("RemoveBuy received: exchange=" + (*args).ExchangeName + "; currency=" + (*args).Currency + "; volume=" + floatToString((*args).Volume))
	} else if (*args).EventType == iombase.RemoveSell {
		fmt.Println("RemoveSell received: exchange=" + (*args).ExchangeName + "; currency=" + (*args).Currency + "; volume=" + floatToString((*args).Volume))
	} else if (*args).EventType == iombase.L2SnapshotAsk {
		fmt.Println("L2SnapshotAsk received: exchange=" + (*args).ExchangeName + "; currency=" + (*args).Currency + "; volume=" + floatToString((*args).Volume) + "; price=" + floatToString((*args).Price))
	} else {
		//unrecognized
		defer fmt.Println("Unrecognized input event from IM.")
	}

	return nil
}

//BookPressure external calling designation
func DummyStrategy(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Dummy strategy.")

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

	//connect to any input modules and call RegisterStrategy
	inModules := make([]*rpc.Client, len(iomList))
	for a := 0; a < len(iomList); a++ {
		inModules[a] = nil
		port := strconv.Itoa(iomHighPort - iomList[a]*2 + 1)

		client, err := rpc.DialHTTP("tcp", "localhost:"+port)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Connected to input module at port " + port + ".")
			inModules[a] = client

			//call RegisterStrategy
			args := &iombase.RegisterStrategyArgs{
				StrategyPort: config["strategy-port"],
				StrategyName: config["strategy-name"],
				ListenEvents: nil,
				ListenData:   nil,
			}
			var reply int
			err = client.Call("InputModuleServer.RegisterStrategy", args, &reply)
			if err != nil {
				log.Fatal("RegisterStrategy error:", err)
			}

			fmt.Println("Strategy registered with IM.")
		}
	}

	//TODO: standardize with above
	//connect to any output modules
	outModules := make([]*rpc.Client, len(iomList))
	if config["output-std"] != "yes" {
		//connect to the servers on the list
		for a := 0; a < len(iomList); a++ {
			port := strconv.Itoa(iomHighPort - iomList[a]*2)

			client, err := rpc.DialHTTP("tcp", "localhost:"+port)
			if err != nil {
				log.Fatal("dialing:", err)
			}
			fmt.Println("Connected to output module at port " + port)

			outModules[a] = client
		}
	}

	//setup timed function calls
	stratTimer, err := strconv.Atoi(config["strat-timer"])
	if err != nil {
		log.Fatal("strat-timer configuration:", err)
	}
	ticker := time.NewTicker(time.Duration(stratTimer) * time.Millisecond)
	tickerQuit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				onStratTimer()
			case <-tickerQuit:
				ticker.Stop()
				return
			}
		}
	}()

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
			close(tickerQuit)
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit'.")
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

func floatToString(inputNum float64) string {
	// to convert a float number to a string
	return strconv.FormatFloat(inputNum, 'f', 6, 64)
}

//called on an interval
func onStratTimer() {
	fmt.Println("Timed function called.")

	//process orderbook state here
}
