package iombase

//ExchangeEvent docs
type ExchangeEvent int

//enum
const (
	_          ExchangeEvent = iota
	PlaceBuy                 // a new buy order was placed
	PlaceSell                // a new sell order was placed
	RemoveBuy                // an order has be removed from the orderbook
	RemoveSell               // sell order removed
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
)

//RegisterStrategyArgs read the docs
type RegisterStrategyArgs struct {
	StrategyPort string
	StrategyName string
	ListenEvents []ExchangeEvent
	ListenData   []ExchangeDatum
}
