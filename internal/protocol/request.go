package protocol

import "context"

type Session interface {
	ServerSession
	ID() string
}

type Request interface {
	GetSession() Session
	GetPayload() any
}

type ServerSession interface {
	Send(ctx context.Context, result any) (any, error)
}
