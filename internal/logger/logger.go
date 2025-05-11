package logger

import (
	"log"
	"os"

	"go_vk/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetupLogger(cfg config.LoggerConfig) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		log.Printf("Invalid log level '%s', using 'info'. Error: %v", cfg.Level, err)
		level = zapcore.InfoLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	if cfg.Format == "text" || cfg.Format == "console" {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	var output zapcore.WriteSyncer
	if cfg.Output == "stdout" {
		output = zapcore.AddSync(os.Stdout)
	} else if cfg.Output == "stderr" {
		output = zapcore.AddSync(os.Stderr)
	} else {
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file '%s', using stdout. Error: %v", cfg.Output, err)
			output = zapcore.AddSync(os.Stdout)
		} else {
			output = zapcore.AddSync(file)
		}
	}

	core := zapcore.NewCore(encoder, output, level)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
