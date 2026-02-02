package platform

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
	ChatID    int64
	Text      string
	ParseMode string
}

const (
	ParseModeHTML = "HTML"
	ActionTyping  = "typing"
)

type Platform interface {
	Name() string
	Listen(handler func(IncomingMessage) error) error
	Send(msg OutgoingMessage) error
	SendAction(chatID int64, action string) error
	Stop()
}
