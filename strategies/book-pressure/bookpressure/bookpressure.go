package bookpressure

import (
	"bufio"
	"container/heap"
	"container/list"
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
)

//TODO: import this from iom packages
type exchangeEvent int

//enum
const (
	Unknown    exchangeEvent = iota
	PlaceBuy                 // a new buy order was placed
	PlaceSell                // a new sell order was placed
	RemoveBuy                // an order has be removed from the orderbook
	RemoveSell               // sell order removed
)

//StrategyServer int-equivalent error code return type
type StrategyServer int

//OnInputEventArgs argument type for OnInputEvent
type OnInputEventArgs struct {
	exchangeName string //TODO: don't ignore this
	eventType    exchangeEvent
	currency     string //TODO: actually use this
	volume       float64
}

//F64Heap gloabl heap var to keep track of orderbook
type F64HItem struct {
	value float64
	index int
}

type F64Heap []F64HItem

func (h F64Heap) Len() int {
	return len(h)
}
func (h F64Heap) Less(i, j int) bool {
	return h[i].value < h[j].value
}
func (h F64Heap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = j
	h[j].index = i
}

func (h *F64Heap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	item := F64HItem{
		value: x.(float64),
		index: len(*h),
	}
	*h = append(*h, item)
}

func (h *F64Heap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

//orderbook for buy/sell limit orders mapped by currency->orders
var buyOrderbook map[string]*F64Heap
var sellOrderbook map[string]*F64Heap

//complimentary structures for orderbook heaps to locate elements by value in heap
var revBuyOB map[string]*map[float64]*list.List
var revSellOB map[string]*map[float64]*list.List

//OnInputEvent called by IOM when input event has arrived
func (t *StrategyServer) OnInputEvent(args *OnInputEventArgs, reply *int) error {
	//init F64Heap for this currency if it doesn't exist yet
	if _, ok := buyOrderbook[(*args).currency]; !ok {
		buyOrderbook[(*args).currency] = &F64Heap{}
		//TODO: init map ptr
	}
	if _, ok := buyOrderbook[(*args).currency]; !ok {
		sellOrderbook[(*args).currency] = &F64Heap{}
		//TODO: init map ptr
	}

	if (*args).eventType == PlaceBuy {
		fmt.Println("PlaceBuy received.")

		relOB := buyOrderbook[(*args).currency]
		heap.Push(relOB, (*args).volume)
		(*revBuyOB[(*args).currency])[(*args).volume].PushBack(&((*relOB)[relOB.Len()-1]))
	} else if (*args).eventType == PlaceSell {
		fmt.Println("PlaceSell received.")

		relOB := sellOrderbook[(*args).currency]
		heap.Push(relOB, (*args).volume)
		(*revSellOB[(*args).currency])[(*args).volume].PushBack(&((*relOB)[relOB.Len()-1]))
	} else if (*args).eventType == RemoveBuy {
		fmt.Println("RemoveBuy received.")

		relOB := buyOrderbook[(*args).currency]
		relRevOB := (*revBuyOB[(*args).currency])[(*args).volume]
		f64HItemRef := relRevOB.Front()
		castOF64HI, ok := (*f64HItemRef).Value.(*F64HItem)
		if !ok {
			fmt.Println("Interface type assertion went wrong in OnInputEvent.")
			panic(-1)
		}
		heap.Remove(relOB, (*castOF64HI).index)
		relRevOB.Remove(f64HItemRef)
	} else if (*args).eventType == RemoveSell {
		fmt.Println("RemoveSell received.")

		relOB := sellOrderbook[(*args).currency]
		relRevOB := (*revSellOB[(*args).currency])[(*args).volume]
		f64HItemRef := relRevOB.Front()
		castOF64HI, ok := (*f64HItemRef).Value.(*F64HItem)
		if !ok {
			fmt.Println("Interface type assertion went wrong in OnInputEvent.")
			panic(-1)
		}
		heap.Remove(relOB, (*castOF64HI).index)
		relRevOB.Remove(f64HItemRef)
	} else {
		//unrecognized
		defer fmt.Println("Unrecognized input event from IM.")
	}

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

//called on an interval
func onStratTimer() {
	fmt.Println("Timed function called.")

	//process orderbook state here
}
