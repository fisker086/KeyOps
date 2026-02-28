package logger

import (
	"os"

	"github.com/fisker086/keyops/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger 全局日志实例
	Logger *zap.Logger
	// SugaredLogger 带语法糖的日志实例（支持格式化）
	Sugar *zap.SugaredLogger
)

// Init 初始化日志系统（仅输出到控制台）
func Init(cfg *config.LoggingConfig) error {
	level := parseLevel(cfg.Level)

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)
	Logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	Sugar = Logger.Sugar()
	zap.ReplaceGlobals(Logger)

	Sugar.Infof("Logger initialized: level=%s", cfg.Level)
	return nil
}

// parseLevel 解析日志级别
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Debug 调试级别日志
func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
	}
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Debugf(format, args...)
	}
}

// Info 信息级别日志
func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
	}
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Infof(format, args...)
	}
}

// Warn 警告级别日志
func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Warnf(format, args...)
	}
}

// Error 错误级别日志
func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Errorf(format, args...)
	}
}

// Fatal 致命错误日志（会退出程序）
func Fatal(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	}
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	if Sugar != nil {
		Sugar.Fatalf(format, args...)
	}
}

// Sync 刷新缓冲区
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// With 创建带字段的子 logger
func With(fields ...zap.Field) *zap.Logger {
	if Logger != nil {
		return Logger.With(fields...)
	}
	return nil
}
