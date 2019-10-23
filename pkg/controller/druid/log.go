package druid

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	golog "log"
	"sync"
)

var (
	logLevelPtr   *int
	sugaredLogger *loggerT
	onceInLog     sync.Once
)

func init() {
	logLevelPtr = flag.Int("logLevel", int(zapcore.InfoLevel), fmt.Sprintf("int value [%d - %d]", zapcore.DebugLevel, zapcore.FatalLevel))

}

type loggerT struct {
	*zap.SugaredLogger
}

func (l *loggerT) IsDebugEnabled() bool {
	return l.Desugar().Core().Enabled(zapcore.DebugLevel)
}

func getLogger() *loggerT {
	onceInLog.Do(initZapLogger)
	return sugaredLogger
}

func initZapLogger() {
	if !flag.Parsed() {
		golog.Panic("Can't get zap logger before flags have been parsed.")
	}

	zapConfig := zap.NewProductionConfig()
	zapConfig.DisableStacktrace = true
	zapConfig.Encoding = "console"

	logLevel := zapcore.Level(*logLevelPtr)
	zapConfig.Level = zap.NewAtomicLevelAt(logLevel)
	golog.Printf("zap logger is configured to print logs at Level[%s].", logLevel.String())

	if tmp, err := zapConfig.Build(); err != nil {
		golog.Panic("Failed to initialize zap logger.")
	} else {
		golog.Println("Successfully created zap logger.")
		sugaredLogger = &loggerT{tmp.Sugar()}
	}
}
