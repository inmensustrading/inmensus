package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tkanos/gonfig"
	"github.com/ziutek/mymysql/mysql"
	_ "github.com/ziutek/mymysql/native" //native engine
)

type configType struct {
	WebsocketURL         string
	WebsocketParams      []string
	ChangeDBUpdateMS     int
	CheckpointDBUpdateMS int //currently 12 hours and on restart, obviously
	ConnectRetryMS       int
	MySQLEndpoint        string
	MySQLIP              string
	MySQLUsername        string
	MySQLPassword        string
	DatabaseName         string
	ChangeEventsTable    string
	CheckpointsTable     string
	CkpTimesTable        string
}

type changeDBRowType struct {
	time int64
	side uint8
	/*
		0: bid
		1: ask
	*/
	price     float64
	remaining float64
	reason    uint8
	/*
		0: connect
		1: disconnect
		2: place
		3: trade
		4: cancel
		5: initial
	*/
}

var config configType
var changeDBBuffer bytes.Buffer

//ModuleMain external calling designation
func main() {
	//unset flag to terminate connection peacefully
	var wantWSConnect = true

	//init
	fmt.Println("Starting...")
	fmt.Println("Gemini: Change Monitor & Checkpointer.")

	//read config
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Configuration: No caller information.")
	}
	fmt.Println("File directory: ", path.Dir(filename))
	err := gonfig.GetConf(path.Dir(filename)+"/conf.json", &config)
	if err != nil {
		panic(err)
	}
	fmt.Println("Configuration: ", config)

	//connect to db
	fmt.Println("Connecting to database...")
	dbConn := mysql.New("tcp", "", config.MySQLEndpoint+":3306", config.MySQLUsername, config.MySQLPassword, config.DatabaseName)
	err = dbConn.Connect()
	if err != nil {
		panic(err)
	}

	//setup websocket listening
	addParams := "?"
	for _, elem := range config.WebsocketParams {
		addParams += elem + "&"
	}
	addParams = strings.TrimSuffix(addParams, "&")
	//only monitors btcusd for now
	wsEndpoint := config.WebsocketURL + "btcusd" + addParams
	wsConn := connectWebsocket(&dbConn, wsEndpoint)

	//message handling from websocket
	msgProcDone := make(chan bool)
	go func() {
		fmt.Println("Message handling initiated.")
		wsMsgProc(&dbConn, wsConn, wsEndpoint, &wantWSConnect)
		fmt.Println("Message handling terminated.")
		msgProcDone <- true
	}()

	//timed updates to change db
	updateChangeDBTicker := time.NewTicker(time.Duration(config.ChangeDBUpdateMS) * time.Millisecond)
	updateChangeDBQuit := make(chan struct{})
	updateChangeDBDone := make(chan bool)
	go func() {
		fmt.Println("Change table updates initiated.")
		updateChangeDB(&dbConn)
		for {
			select {
			case <-updateChangeDBTicker.C:
				updateChangeDB(&dbConn)
			case <-updateChangeDBQuit:
				updateChangeDBTicker.Stop()
				fmt.Println("Change table updates terminated.")
				updateChangeDBDone <- true
				return
			}
		}
	}()

	//timed updates to checkpoints db
	updateCheckpointDBTicker := time.NewTicker(time.Duration(config.CheckpointDBUpdateMS) * time.Millisecond)
	updateCheckpointDBQuit := make(chan struct{})
	updateCheckpointDBDone := make(chan bool)
	go func() {
		fmt.Println("Checkpoint table updates initiated.")
		updateCheckpointDB(&dbConn, wsEndpoint)
		for {
			select {
			case <-updateCheckpointDBTicker.C:
				updateCheckpointDB(&dbConn, wsEndpoint)
			case <-updateCheckpointDBQuit:
				updateCheckpointDBTicker.Stop()
				fmt.Println("Checkpoint table updates terminated.")
				updateCheckpointDBDone <- true
				return
			}
		}
	}()

	//setup command loop to exit on 'exit'
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Accepting commands...")
		text, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		text = strings.TrimSpace(text)

		if text == "exit" {
			fmt.Println("Please wait while all threads terminate...")

			//set flag that we don't want to reconnect
			wantWSConnect = false

			//cleanly exit monitoring
			err := wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println(err)
			}

			//manually close connection here
			wsConn.Close()

			//wait for functions to respond and exit
			close(updateChangeDBQuit)
			close(updateCheckpointDBQuit)
			<-updateChangeDBDone
			<-updateCheckpointDBDone

			<-msgProcDone
			break
		} else if text == "help" {
			fmt.Println("Available commands: 'exit', 'test-event', 'count'.")
		} else {
			fmt.Println("Unrecognized command.")
		}
	}

	//conclude
	fmt.Println("Program terminated.")
}

func connectWebsocket(dbConn *mysql.Conn, address string) *websocket.Conn {
	fmt.Println("Connecting to websocket endpoint...")

	c, _, err := websocket.DefaultDialer.Dial(address, nil)
	for err != nil {
		time.Sleep(time.Duration(config.ConnectRetryMS) * time.Millisecond)
		c, _, err = websocket.DefaultDialer.Dial(address, nil)
	}

	//everytime we connect, update the db with a reconnect message
	query := fmt.Sprintf(
		"INSERT INTO "+config.ChangeEventsTable+" VALUES (%d,%d,%f,%f,%d);",
		time.Now().UnixNano()/int64(time.Millisecond),
		0,
		0.0,
		0.0,
		0) //reason is connect
	fmt.Println(query)
	_, _, err = (*dbConn).Query(query)
	if err != nil {
		fmt.Println("Error while querying table for connect event:", err)
	}

	fmt.Println("Websocket connected.")
	return c
}

func wsMsgProc(dbConn *mysql.Conn, wsConn *websocket.Conn, wsEndpoint string, wantWSConnect *bool) {
	for {
		//blocks
		_, message, err := wsConn.ReadMessage()

		//TODO: check connection again
		if err != nil {
			fmt.Println("Connection lost: ", err)

			//post an event to the DB for disconnect
			query := fmt.Sprintf(
				"INSERT INTO "+config.ChangeEventsTable+" VALUES (%d,%d,%f,%f,%d);",
				time.Now().UnixNano()/int64(time.Millisecond),
				0,
				0.0,
				0.0,
				1) //reason is disconnect
			fmt.Println(query)
			_, _, err = (*dbConn).Query(query)
			if err != nil {
				fmt.Println("Error while querying table for disconnect event:", err)
			}

			if *wantWSConnect {
				wsConn = connectWebsocket(dbConn, wsEndpoint)
			} else {
				//if the flag is unset, then the user has manually terminated the program
				break
			}
		} else {
			onWSMessage(message)
		}
	}
}

func onWSMessage(message []byte) {
	var response map[string]interface{}
	err := json.Unmarshal(message, &response)
	if err != nil {
		fmt.Println("Error while unmarshalling message:", err)
		return
	}

	if response["type"].(string) != "update" {
		fmt.Println("Not update type.")
		return
	}

	update := changeDBRowType{}
	if response["timestampms"] == nil {
		//probably initial event
		//set timestamp to be current time
		update.time = time.Now().UnixNano() / int64(time.Millisecond)
	} else {
		update.time = int64(response["timestampms"].(float64))
	}

	events := response["events"].([]interface{})
	for a := 0; a < len(events); a++ {
		cur := events[a].(map[string]interface{})
		if cur["type"].(string) != "change" {
			continue
		}

		side := cur["side"].(string)
		if side == "bid" {
			update.side = 0
		} else if side == "ask" {
			update.side = 1
		} else {
			fmt.Println("Unexpected \"side\" value:", side)
			continue
		}

		update.price, err = strconv.ParseFloat(cur["price"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"price\" value:", cur["price"].(string))
			continue
		}

		update.remaining, err = strconv.ParseFloat(cur["remaining"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"remaining\" value:", cur["remaining"].(string))
			continue
		}

		reason := cur["reason"].(string)
		if reason == "place" {
			update.reason = 2
		} else if reason == "trade" {
			update.reason = 3
		} else if reason == "cancel" {
			update.reason = 4
		} else if reason == "initial" {
			update.reason = 5
		} else {
			fmt.Println("Unexpected \"reason\" value:", reason)
			continue
		}

		changeDBBuffer.WriteString(fmt.Sprintf("(%d,%d,%f,%f,%d),",
			update.time,
			update.side,
			update.price,
			update.remaining,
			update.reason))
	}
}

func updateChangeDB(dbConn *mysql.Conn) {
	if changeDBBuffer.Len() == 0 {
		fmt.Printf(time.Now().Format(time.RFC3339)+": Updated change table with %d bytes...\r", 0)
		return
	}

	queryPart := changeDBBuffer.Bytes()
	queryPart[len(queryPart)-1] = ';'
	query := "INSERT INTO " + config.ChangeEventsTable + " VALUES " + string(queryPart)
	changeDBBuffer.Reset()

	_, _, err := (*dbConn).Query(query)
	if err != nil {
		fmt.Println("Error while querying change table:", err)
	}

	fmt.Printf(time.Now().Format(time.RFC3339)+": Updated change table with %d bytes...\r", len(query))
}

func updateCheckpointDB(dbConn *mysql.Conn, wsEndpoint string) {
	//briefly connect to the websocket endpoint, get the orderbook state, then disconnect and push the orderbook as a checkpoint to the database
	wsConn, _, err := websocket.DefaultDialer.Dial(wsEndpoint, nil)
	//retry a maximum of 8 times
	retries := 0
	for err != nil && retries < 8 {
		time.Sleep(time.Duration(config.ConnectRetryMS) * time.Millisecond)
		wsConn, _, err = websocket.DefaultDialer.Dial(wsEndpoint, nil)
		retries++
	}

	//listen only for initial message
	_, message, err := wsConn.ReadMessage()
	if err != nil {
		fmt.Println("Checkpoint could not be made:", err)
		wsConn.Close()
		return
	}

	//break down message
	var response map[string]interface{}
	err = json.Unmarshal(message, &response)
	if err != nil {
		fmt.Println("Error while unmarshalling message:", err)
		return
	}

	if response["type"].(string) != "update" {
		fmt.Println("Not update type.")
		return
	}

	//contains query to checkpoints db
	var buffer bytes.Buffer
	update := changeDBRowType{}
	if response["timestampms"] == nil {
		//probably initial event
		//set timestamp to be current time
		update.time = time.Now().UnixNano() / int64(time.Millisecond)
	} else {
		update.time = int64(response["timestampms"].(float64))
	}

	events := response["events"].([]interface{})
	for a := 0; a < len(events); a++ {
		cur := events[a].(map[string]interface{})
		if cur["type"].(string) != "change" {
			continue
		}

		side := cur["side"].(string)
		if side == "bid" {
			update.side = 0
		} else if side == "ask" {
			update.side = 1
		} else {
			fmt.Println("Unexpected \"side\" value:", side)
			continue
		}

		update.price, err = strconv.ParseFloat(cur["price"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"price\" value:", cur["price"].(string))
			continue
		}

		update.remaining, err = strconv.ParseFloat(cur["remaining"].(string), 64)
		if err != nil {
			fmt.Println("Unexpected \"remaining\" value:", cur["remaining"].(string))
			continue
		}

		reason := cur["reason"].(string)
		if reason == "place" {
			update.reason = 2
		} else if reason == "trade" {
			update.reason = 3
		} else if reason == "cancel" {
			update.reason = 4
		} else if reason == "initial" {
			update.reason = 5
		} else {
			fmt.Println("Unexpected \"reason\" value:", reason)
			continue
		}

		//only accept initial updates
		if update.reason != 5 {
			fmt.Println("Not initial update:", reason)
			continue
		}

		buffer.WriteString(fmt.Sprintf("(%d,%d,%f,%f,%d),",
			update.time,
			update.side,
			update.price,
			update.remaining,
			update.reason))
	}

	wsConn.Close()

	//update the checkpoint times table with the time, then update the checkpoints table with all the initial events
	fmt.Println("Creating checkpoint...")

	//checkpoints time table
	_, _, err = (*dbConn).Query("INSERT INTO "+config.CkpTimesTable+" VALUES (%d)", update.time)
	if err != nil {
		fmt.Println("Error while querying checkpoint times table:", err)
		return
	}

	//checkpoint
	if buffer.Len() == 0 {
		fmt.Println("Error: No checkpoint?")
	} else {
		queryPart := buffer.Bytes()
		queryPart[len(queryPart)-1] = ';'
		_, _, err = (*dbConn).Query("INSERT INTO " + config.CheckpointsTable + " VALUES " + string(queryPart))
		if err != nil {
			fmt.Println("Error while querying checkpoints table:", err)
		}

		fmt.Printf(time.Now().Format(time.RFC3339)+": Checkpoint created with %d bytes.\n", len(queryPart))
	}
}
