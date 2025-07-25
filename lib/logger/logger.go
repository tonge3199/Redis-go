// Package logger 提供高性能的日志记录功能
// 支持控制台输出、文件输出、日志轮转、异步写入等特性
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Settings stores config for Logger.
// Settings 存储日志配置信息
type Settings struct {
	Path       string `yaml:"path"`        // 日志文件路径
	Name       string `yaml:"name"`        // 日志文件名前缀
	Ext        string `yaml:"ext"`         // 文件扩展名
	TimeFormat string `yaml:"time-format"` // TimeFormat: 时间格式模板（用于日志轮转）
}

type LogLevel int

// Output levels
const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

const (
	flags              = log.LstdFlags // 标准日志格式标志
	defaultCallerDepth = 2             // 调用栈深度（跳过包装函数）
	bufferSize         = 1e5           // 日志缓冲区大小（10万条）
)

// logEntry 日志条目结构体
// 用于异步日志写入时的数据传输
type logEntry struct {
	msg   string
	level LogLevel
}

var (
	// levelFlags 日志级别字符串映射
	levelFlags = []string{"DEBUG", "INFO", "WARNING", "ERROR", "FATAL"}
)

// ILogger defines the methods that any logger should implement
// ILogger 日志接口定义 定义了所有日志器必须实现的方法
type ILogger interface {
	Output(level LogLevel, callerDepth int, msg string)
}

// Logger is Logger
// Logger 日志器结构体
// 实现了高性能的异步日志记录
// logFile: 日志文件句柄
// logger: 标准库日志器实例
// entryChan: 日志条目通道（异步写入）
// entryPool: 对象池，复用logEntry对象
type Logger struct {
	logFile   *os.File
	logger    *log.Logger
	entryChan chan *logEntry
	entryPool *sync.Pool
}

// DefaultLogger 默认日志器实例
// 默认使用标准输出日志器
var DefaultLogger ILogger = NewStdoutLogger()

// NewStdoutLogger 创建标准输出日志器
// 返回一个将日志输出到控制台的Logger实例
// 使用异步goroutine处理日志写入，避免阻塞主流程
// 使用对象池减少内存分配压力
func NewStdoutLogger() *Logger {
	logger := &Logger{
		logFile:   nil,
		logger:    log.New(os.Stdout, "", flags),
		entryChan: make(chan *logEntry, bufferSize),
		entryPool: &sync.Pool{
			New: func() any { // any is interface{}
				return &logEntry{}
			},
		},
	}
	// 启动异步日志处理goroutine
	// 从通道读取日志条目并写入输出
	go func() {
		for e := range logger.entryChan {
			// msg includes call stack, no need for calldepth
			// msg 已包含调用栈，无需 calldepth
			_ = logger.logger.Output(0, e.msg)
			logger.entryPool.Put(e) // 归还对象到池
		}
	}()
	return logger
}

// NewFileLogger creates a logger which print msg to stdout and log file
// NewFileLogger 创建文件日志器
// 创建同时输出到控制台和文件的日志器
// 支持基于时间的日志轮转功能
// 参数：
//
//	settings: 日志配置，包含路径、文件名、扩展名等
//
// 返回值：
//
//	*Logger: 日志器实例
//	error: 创建过程中的错误
func NewFileLogger(settings *Settings) (*Logger, error) {
	fileName := fmt.Sprintf("%s-%s.%s",
		settings.Name,
		time.Now().Format(settings.TimeFormat),
		settings.Ext)
	logFile, err := mustOpen(fileName, settings.Path)
	if err != nil {
		return nil, fmt.Errorf("logging.Join err: %s", err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	logger := &Logger{
		logFile:   logFile,
		logger:    log.New(mw, "", flags),
		entryChan: make(chan *logEntry, bufferSize),
		entryPool: &sync.Pool{
			New: func() any {
				return &logEntry{}
			},
		},
	}
	go func() {
		for e := range logger.entryChan {
			logFilename := fmt.Sprintf("%s-%s.%s",
				settings.Name,
				time.Now().Format(settings.TimeFormat),
				settings.Ext)
			if path.Join(settings.Path, logFilename) != logger.logFile.Name() {
				logFile, err := mustOpen(logFilename, settings.Path)
				if err != nil {
					panic("open log " + logFilename + " failed: " + err.Error())
				}
				logger.logFile = logFile
				logger.logger = log.New(io.MultiWriter(os.Stdout, logFile), "", flags)
			}
			_ = logger.logger.Output(0, e.msg) // msg includes call stack, no need for calldepth
			logger.entryPool.Put(e)
		}
	}()
	return logger, nil
}

// Setup initializes DefaultLogger
// Setup 初始化默认日志器
// 使用文件日志器替换默认的标准输出日志器
// 通常在程序启动时调用一次
// 参数：
//
//	settings: 日志配置信息
func Setup(settings *Settings) {
	logger, err := NewFileLogger(settings)
	if err != nil {
		panic(err)
	}
	DefaultLogger = logger
}

// Output sends a msg to logger
// Output 输出日志消息
// Logger的核心方法，所有日志输出都通过此方法
// 参数：
//
//	level: 日志级别
//	callerDepth: 调用栈深度（用于定位日志来源）
//	msg: 原始日志消息
func (logger *Logger) Output(level LogLevel, callerDepth int, msg string) {
	var formattedMsg string

	// 获取调用栈信息
	_, file, line, ok := runtime.Caller(callerDepth)
	if ok {
		// 格式化日志消息，包含级别、文件名、行号和消息内容
		formattedMsg = fmt.Sprintf("[%s][%s:%d] %s", levelFlags[level], filepath.Base(file), line, msg)
	} else {
		// 无法获取调用栈信息时的降级处理
		formattedMsg = fmt.Sprintf("[%s] %s", levelFlags[level], msg)
	}
	// 从对象池获取logEntry对象
	entry := logger.entryPool.Get().(*logEntry) // 类型断言确定为*logEntry
	entry.level = level
	entry.msg = formattedMsg

	// 发送到异步处理通道
	logger.entryChan <- entry
}

// Debug logs debug message through DefaultLogger
// Debug 输出调试级别日志
// 使用默认日志器输出调试信息
// 参数：可变参数，会被格式化为字符串
func Debug(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	DefaultLogger.Output(DEBUG, defaultCallerDepth, msg)
}

// Debugf logs debug message through DefaultLogger
// Debugf 格式化输出调试级别日志
// 使用默认日志器输出格式化调试信息
// 参数：
//
//	format: 格式字符串
//	v: 格式化参数
func Debugf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	DefaultLogger.Output(DEBUG, defaultCallerDepth, msg)
}

// Info logs message through DefaultLogger
// Info 输出信息级别日志
// 使用默认日志器输出普通信息
func Info(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	DefaultLogger.Output(INFO, defaultCallerDepth, msg)
}

// Infof logs message through DefaultLogger
// Infof 格式化输出信息级别日志
// 使用默认日志器输出格式化信息
func Infof(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	DefaultLogger.Output(INFO, defaultCallerDepth, msg)
}

// Warn logs warning message through DefaultLogger
func Warn(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	DefaultLogger.Output(WARNING, defaultCallerDepth, msg)
}

// Error logs error message through DefaultLogger
func Error(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	DefaultLogger.Output(ERROR, defaultCallerDepth, msg)
}

// Errorf logs error message through DefaultLogger
func Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	DefaultLogger.Output(ERROR, defaultCallerDepth, msg)
}

// Fatal prints error message then stop the program
func Fatal(v ...interface{}) {
	msg := fmt.Sprintln(v...)
	DefaultLogger.Output(FATAL, defaultCallerDepth, msg)
}
