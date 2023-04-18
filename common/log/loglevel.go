package log

import (
	"fmt"
)

type Level byte

const (
	Debug Level = iota
	Info
	Warning
	Error
	None
)

type Prefix = string

func NewPrefix(tag Prefix) Prefix {
	return fmt.Sprintf("[%s] ", tag)
}
