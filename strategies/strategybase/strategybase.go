package strategybase

import (
	"../../io-modules/iombase"
)

//OnInputEventArgs argument type for OnInputEvent
type OnInputEventArgs struct {
	ExchangeName string //TODO: don't ignore this
	EventType    iombase.ExchangeEvent
	Currency     string //TODO: actually use this
	Volume       float64
}
