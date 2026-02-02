package provider

type AIProvider interface {
	Name() string
	Generate(prompt string, model string) (string, error)
}
