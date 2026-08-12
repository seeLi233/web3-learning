package logger

import (
	"os"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log *zap.Logger
)

// Init 初始化全局日志
// logPath： 日志文件路径
// level：日志级别，debug/info/warn/error
func Init(logPath string, level string) {
	// 1.日志切割配置
	rotter := &lumberjack.Logger{
		Filename:   logPath, // 日志文件路径
		MaxSize:    100,     // 每个日志文件最大100MB
		MaxBackups: 30,      // 最多保留30个旧日志文件
		MaxAge:     7,       // 最多保留7天的日志
		Compress:   true,    // 是否压缩旧日志文件
		LocalTime:  true,
	}

	// 2.定义写入器：文件+控制台
	fileWriter := zapcore.AddSync(rotter)
	consoleWriter := zapcore.AddSync(os.Stdout)

	// 3.时间格式
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 控制台彩色级别

	// 4.编码器
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)       // 文件JSON格式
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig) // 控制台文本格式

	// 5.日志级别
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zap.DebugLevel
	case "info":
		zapLevel = zap.InfoLevel
	case "warn":
		zapLevel = zap.WarnLevel
	case "error":
		zapLevel = zap.ErrorLevel
	default:
		zapLevel = zap.InfoLevel
	}

	// 6.多输出核心
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, fileWriter, zapLevel),       // 文件核心
		zapcore.NewCore(consoleEncoder, consoleWriter, zapLevel), // 控制台核心
	)

	// 7.构建全局 logger
	Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

// Sync 刷新日志缓冲区, 程序退出调用
func Sync() {
	_ = Log.Sync()
}

// 快捷方法
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}
