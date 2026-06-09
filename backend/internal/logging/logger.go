package logging

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Logger struct {
	logger *log.Logger
}

func New() *Logger {
	return &Logger{logger: log.New(os.Stdout, "", 0)}
}

func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.write("info", message, fields)
}

func (l *Logger) Error(message string, fields map[string]interface{}) {
	l.write("error", message, fields)
}

func (l *Logger) write(level, message string, fields map[string]interface{}) {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["level"] = level
	fields["message"] = message
	fields["time"] = time.Now().Format(time.RFC3339Nano)
	payload, err := json.Marshal(fields)
	if err != nil {
		l.logger.Printf("%s %s", level, message)
		return
	}
	l.logger.Println(string(payload))
}
