package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

// Logger 日志接口
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// loggerImpl 日志实现
type loggerImpl struct {
	level      LogLevel
	logger     *log.Logger
	fileWriter io.Writer
	consoleOut io.Writer
	enableFile bool
	enableConsole bool
}

var defaultLogger Logger

// InitLogger 初始化日志系统
func InitLogger(config *Config) (Logger, error) {
	var writers []io.Writer
	
	// 控制台输出
	if config.EnableConsole {
		writers = append(writers, os.Stdout)
	}
	
	// 文件输出
	var fileWriter io.Writer
	if config.EnableFile {
		logDir := config.LogDir
		if logDir == "" {
			logDir = "logs"
		}
		
		// 确保日志目录存在
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		
		// 生成日志文件名
		var logFile string
		if config.LogFile != "" {
			logFile = filepath.Join(logDir, config.LogFile)
		} else {
			logFile = filepath.Join(logDir, fmt.Sprintf("cloudbot-%s.log", time.Now().Format("2006-01-02")))
		}
		
		// 打开日志文件（追加模式）
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		
		fileWriter = file
		writers = append(writers, fileWriter)
	}
	
	// 创建多写入器
	multiWriter := io.MultiWriter(writers...)
	
	// 创建日志实例
	logger := &loggerImpl{
		level:        config.Level,
		logger:       log.New(multiWriter, "", 0),
		fileWriter:   fileWriter,
		consoleOut:   os.Stdout,
		enableFile:   config.EnableFile,
		enableConsole: config.EnableConsole,
	}
	
	defaultLogger = logger
	return logger, nil
}

// GetLogger 获取默认日志实例
func GetLogger() Logger {
	if defaultLogger == nil {
		// 如果没有初始化，创建一个默认的控制台日志
		config := &Config{
			Level:        INFO,
			EnableConsole: true,
			EnableFile:   false,
		}
		logger, _ := InitLogger(config)
		return logger
	}
	return defaultLogger
}

// SetLevel 设置日志级别
func (l *loggerImpl) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel 获取日志级别
func (l *loggerImpl) GetLevel() LogLevel {
	return l.level
}

// log 内部日志方法
func (l *loggerImpl) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}
	
	// 获取调用者信息
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	} else {
		// 只保留文件名
		file = filepath.Base(file)
	}
	
	// 格式化消息
	message := fmt.Sprintf(format, args...)
	
	// 格式化日志行
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]
	logLine := fmt.Sprintf("[%s] [%s] [%s:%d] %s", timestamp, levelName, file, line, message)
	
	// 写入日志
	l.logger.Println(logLine)
}

// Debug 调试日志
func (l *loggerImpl) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *loggerImpl) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *loggerImpl) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *loggerImpl) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// ParseLevel 解析日志级别字符串
func ParseLevel(levelStr string) LogLevel {
	levelStr = strings.ToUpper(strings.TrimSpace(levelStr))
	switch levelStr {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	return levelNames[l]
}

