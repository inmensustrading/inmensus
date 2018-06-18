package iombase

//ExchangeEvent docs
type ExchangeEvent int

//enum
const (
	_             ExchangeEvent = iota
	PlaceBuy                    // a new buy order was placed
	PlaceSell                   // a new sell order was placed
	RemoveBuy                   // an order has be removed from the orderbook
	RemoveSell                  // sell order removed
	L2SnapshotAsk               // a snapshot of the asks in the order book
)

//ExchangeDatum docs
type ExchangeDatum int

//enum
const (
	_            ExchangeDatum = iota //TODO: fix _ perhaps
	ExchangeName                      // identifier of the exchange related to the event
	EventType                         // one of the identifiers specified under the "listen-events" key
	Currency                          // identifier for the type of currency assciated with the event
	Volume                            // amount of currency associated with the event
	Price                             // the value of one unit of currency, not actually sure
	// how we know which unit it's in besides examination (btc vs usd)
)

//RegisterStrategyArgs read the docs
type RegisterStrategyArgs struct {
	StrategyPort string
	StrategyName string
	ListenEvents []ExchangeEvent
	ListenData   []ExchangeDatum
}

//general purpose tuple
type Ask struct {
	price, size interface{}
}
