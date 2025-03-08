package pretty

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"
)

type CustomLogger struct {
	Level         string
	Output        io.Writer
	DebugInfo     FuncDebugInfo
	PrefixProps   []CustomLoggerPrefixProperty
	PrettyConsole bool
}

type FuncDebugInfo struct {
	FunctionName string `json:"functionName"`
	FileName     string `json:"fileName"`
	LineNumber   int    `json:"lineNumber"`
}

type CustomLoggerPrefixProperty struct {
	Value     string `json:"value"`
	Index     int8   `json:"index"`
	Padding   int8   `json:"padding"`
	Seperator string `json:"seperator"`
}

func NewCustomLogger(output io.Writer, level string, padding int8, seperatorChar string, pretty bool) *CustomLogger {
	debugInfo := &FuncDebugInfo{}
	prefixPropDate := &CustomLoggerPrefixProperty{
		Value:     DateTimeSting(time.Now()),
		Index:     int8(0),
		Padding:   padding,
		Seperator: seperatorChar,
	}
	props := make([]CustomLoggerPrefixProperty, 0, 8)
	props = append(props, *prefixPropDate)
	logger := &CustomLogger{
		Level:         level,
		Output:        output,
		DebugInfo:     *debugInfo,
		PrefixProps:   props,
		PrettyConsole: true,
	}
	return logger
}

func (lp *CustomLoggerPrefixProperty) GetPaddingString() string {
	var sbPad strings.Builder
	for i := 0; i < int(lp.Padding); i++ {
		sbPad.WriteString(" ")
	}
	padStr := sbPad.String()
	return padStr
}

func (lp *CustomLoggerPrefixProperty) ToString() string {
	var prefixBuilder strings.Builder
	var sbPad strings.Builder
	for i := 0; i < int(lp.Padding); i++ {
		sbPad.WriteString(" ")
	}
	padStr := sbPad.String()
	if lp.Index == 0 {
		prefixBuilder.WriteString(DateTimeSting(time.Now()))
		prefixBuilder.WriteString(padStr)
		prefixBuilder.WriteString(lp.Seperator)
	} else {
		prefixBuilder.WriteString(padStr)
		prefixBuilder.WriteString(lp.Value)
		prefixBuilder.WriteString(padStr)
		prefixBuilder.WriteString(lp.Seperator)
	}
	return prefixBuilder.String()
}

func (c *CustomLogger) Prefix(level string) string {
	var prefixBuilder strings.Builder
	sort.Slice(c.PrefixProps, func(i, j int) bool {
		return c.PrefixProps[i].Index < c.PrefixProps[j].Index
	})

	for idx, v := range c.PrefixProps {
		switch idx {
		case 0:
			prefixBuilder.WriteString(v.GetPaddingString())
			prefixBuilder.WriteString(level)
			prefixBuilder.WriteString(v.GetPaddingString())
			prefixBuilder.WriteString(v.Seperator)
		}
		prefixBuilder.WriteString(v.ToString())
	}

	return prefixBuilder.String()
}

func (l *CustomLogger) Info(message string) {
	msgString := l.Prefix("[INFO]") + " - " + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogInfoString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func (l *CustomLogger) Infof(s string, formatter ...any) {
	message := fmt.Sprintf(s, formatter...)
	msgString := l.Prefix("[INFO]") + " - " + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogInfoString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func (l *CustomLogger) Warning(message string) {
	msgString := l.Prefix("[WARN]") + " " + " - " + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogErrorString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func (l *CustomLogger) Error(message string) {
	msgString := l.Prefix("[ERROR]") + " - " + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogErrorString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func (l *CustomLogger) Errorf(s string, formatter ...any) {
	message := fmt.Sprintf(s, formatter...)
	msgString := l.Prefix("[ERROR]") + " - " + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogErrorString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func (l *CustomLogger) Debug(message string) {
	l.DebugInfo.GetCallerDebugInfo(3)
	msgString := l.Prefix("[DEBUG]") + " - " + l.DebugInfo.GetCallerDebugString(3) + message + "\n"
	if l.PrettyConsole {
		msgString = PrettyLogInfoString(msgString)
	}
	l.Output.Write([]byte(msgString))
}

func getCallerInfo(caller int) (string, string, int) {
	pc, file, line, ok := runtime.Caller(caller)
	if !ok {
		return "", "", 0
	}
	funcName := runtime.FuncForPC(pc).Name()

	return funcName, file, line
}

func GetDebugInfoString(caller int) string {
	pc, file, line, ok := runtime.Caller(caller)
	if !ok {
		return ""
	}
	funcName := runtime.FuncForPC(pc).Name()
	debugLogString := fmt.Sprintf("func: %s line: %d %s", funcName, line, file)

	return debugLogString
}

func GetFuncDebugInfo(caller int) FuncDebugInfo {
	pc, file, line, ok := runtime.Caller(caller)
	if !ok {
		return FuncDebugInfo{}
	}
	funcName := runtime.FuncForPC(pc).Name()

	debugInfo := &FuncDebugInfo{
		FunctionName: funcName,
		FileName:     file,
		LineNumber:   line,
	}

	return *debugInfo
}

func (d *FuncDebugInfo) GetCallerDebugInfo(caller int) {
	funcName, file, line := getCallerInfo(caller)
	d.FileName = file
	d.FunctionName = funcName
	d.LineNumber = line
}

func (d *FuncDebugInfo) GetCallerDebugString(caller int) string {
	debugLogString := fmt.Sprintf(" %s %d %s ", d.FunctionName, d.LineNumber, d.FileName)
	return debugLogString
}
