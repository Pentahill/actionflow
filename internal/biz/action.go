package biz

import (
	"github.com/Pentahill/actionflow/internal/channel"
	"reflect"
)

type ActionHandler interface {
	channel.ActionHandler
	SupportedAction() []reflect.Type
	Name() string
}
