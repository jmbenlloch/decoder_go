package decoder

type Logger interface {
	Info(message string, module string)
	Error(string)
}

var logger Logger

func SetLogger(l Logger) {
	logger = l
}
