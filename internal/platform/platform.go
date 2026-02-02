package platform

import "time"

type IncomingMessage struct {
	Platform  string
	ChatID    int64
	UserID    int64
	Username  string
	Text      string
	IsBot     bool
	IsCommand bool
	Command   string
	Args      string
	Raw       interface{}
}

type OutgoingMessage struct {
	ChatID int64
	Text   string
}

type Platform interface {
	Name() string
	Listen(handler func(IncomingMessage) error) error
	Send(msg OutgoingMessage) error
	Stop()
}
