// Package logger 提供日志管理功能
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/logdotar/sshwarden/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// prefixedEncoder 带前缀的日志编码器
//
// 字段说明：
// - Encoder: 基础编码器

type prefixedEncoder struct {
	zapcore.Encoder
}

// EncodeEntry 编码日志条目
//
// 参数:
// - entry: 日志条目
// - fields: 日志字段
//
// 返回值:
// - *buffer.Buffer: 编码后的日志缓冲区
// - error: 编码过程中的错误
func (e *prefixedEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf, err := e.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return nil, err
	}

	logLine := buf.String()
	buf.Reset()
	buf.AppendString("[sshwarden] " + logLine)

	return buf, nil
}

// New 创建日志器
//
// 参数:
// - cfg: 日志配置
//
// 返回值:
// - *zap.Logger: 日志器实例
// - error: 创建过程中的错误
func New(cfg *config.LogConfig) (*zap.Logger, error) {
	// 确保日志目录存在
	if cfg.Filename != "" {
		logDir := filepath.Dir(cfg.Filename)
		if logDir != "" && logDir != "." {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return nil, fmt.Errorf("创建日志目录失败: %w", err)
			}
		}
	}

	lumberJackLogger := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		LocalTime:  cfg.LocalTime,
		Compress:   cfg.Compress,
	}

	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, err
	}

	consoleEncoder := &prefixedEncoder{
		Encoder: zapcore.NewConsoleEncoder(newConsoleEncoderConfig()),
	}
	fileEncoder := &prefixedEncoder{
		Encoder: zapcore.NewConsoleEncoder(newFileEncoderConfig()),
	}

	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	fileCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(lumberJackLogger),
		zapLevel,
	)

	core := zapcore.NewTee(consoleCore, fileCore)
	logger := zap.New(core, zap.AddCaller())

	return logger, nil
}

// newConsoleEncoderConfig 创建控制台编码器配置
//
// 返回值:
// - zapcore.EncoderConfig: 编码器配置
func newConsoleEncoderConfig() zapcore.EncoderConfig {
	encConfig := zap.NewDevelopmentEncoderConfig()
	encConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
	encConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return encConfig
}

// newFileEncoderConfig 创建文件编码器配置
//
// 返回值:
// - zapcore.EncoderConfig: 编码器配置
func newFileEncoderConfig() zapcore.EncoderConfig {
	encConfig := zap.NewProductionEncoderConfig()
	encConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
	encConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return encConfig
}
