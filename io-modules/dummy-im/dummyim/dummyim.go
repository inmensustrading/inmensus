package dummyim

import (
	"fmt"

	"../../iombase"
)

//InputModuleServer needs this declaration in every IM
type InputModuleServer int

//RegisterStrategy read the docs
func (t *InputModuleServer) RegisterStrategy(argType *iombase.RegisterStrategyArgs, replyType *int) error {
	fmt.Println("Strategy registered!")
	return nil
}

//DummyIM external calling designation
func DummyIM(configPath string) {
	//init
	fmt.Println("Starting...")
	fmt.Println("Dummy IM.")

	//clean up and conclude
	fmt.Println("Exiting...")
}
