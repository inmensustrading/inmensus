package io

import (
	"net/rpc"
	"errors"
)

type InputModuleServer struct {
	ioPort int
	ioId int
}

type RegisterStrategyArgs struct {
	strategyPort int
	strategyName string
	listenEvents []string //Tony said that we might wanna use an enum here, are we in 2008?
	listenData []string
}

//TODO: write docstrings for these functions
func (t *InputModuleServer) RegisterStrategy(argType *RegisterStrategyArgs, replyType *int) error {
	return nil
}

//TODO: write setup function for server




