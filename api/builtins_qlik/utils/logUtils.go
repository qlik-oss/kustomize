package utils

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapWriter struct {
	sugar *zap.SugaredLogger
}

func (zw *zapWriter) Write(p []byte) (n int, err error) {
	zw.sugar.Info(string(p))
	return len(p), nil
}

func GetLogWriter(sugar *zap.SugaredLogger) io.Writer {
	return &zapWriter{sugar}
}

func GetNopLogger() *zap.SugaredLogger {
	sugar := zap.NewNop().Sugar()
	defer sugar.Sync()
	return sugar
}

func GetLogger(pluginName string) *zap.SugaredLogger {
	var (
		logger *zap.Logger
		sugar  *zap.SugaredLogger
		cfg    zap.Config
		level  zapcore.Level
	)
	level = zap.ErrorLevel
	value, exists := os.LookupEnv("QKP_LOG_STDERR_ENABLED")
	if exists {
		level.UnmarshalText([]byte(value))
	}
	cfg = zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(level),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			LevelKey:       "level",
			NameKey:        "logger",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.EpochTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}
	logger, _ = cfg.Build()
	sugar = logger.Sugar()
	defer sugar.Sync()
	return sugar
}
