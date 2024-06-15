package iqlogger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

/*
/////////////////////////////////////////////
/////////////////////////////////////////////
// Function List
/////////////////////////////////////////////

// Log with newline
esLogger.Log(LogLevel, MainMsg, ExtraMsg, ...Option)

// Log with newline
esLogger.Panic(msg...)
esLogger.Fatal(msg...)
esLogger.Error(msg...)
esLogger.Warn(msg...)
esLogger.Info(msg...)
esLogger.Debug(msg...)

// Log with format and newline
esLogger.Panicf(format, arg...)
esLogger.Fatalf(format, arg...)
esLogger.Errorf(format, arg...)
esLogger.Warnf(format, arg...)
esLogger.Infof(format, arg...)
esLogger.Debugf(format, arg...)

// Log with double newline
esLogger.Panicln(msg...)
esLogger.Fatalln(msg...)
esLogger.Errorln(msg...)
esLogger.Warnln(msg...)
esLogger.Infoln(msg...)
esLogger.Debugln(msg...)


// All of above can call WithFields like this
esLogger.WithFields({"A":a, "B":b}).Log(LogLevel, MainMsg, ExtraMsg, ...Option)
esLogger.WithFields({"A":a, "B":b}).Debug(msg...)
esLogger.WithFields({"A":a, "B":b}).Debugf(format, msg...)
esLogger.WithFields({"A":a, "B":b}).Debugln(msg...)

/////////////////////////////////////////////
/////////////////////////////////////////////
*/
// var logrusLogger = logrus.New()

// EFields type, used to pass to `WithFields`.
type EFields map[string]interface{}

type einsteinLoggerOpts struct {
	tis620ToUtf8 bool
}

// OptEinsteinLogger provide options for each operation.
type OptEinsteinLogger func(*einsteinLoggerOpts)

// OptTis620ToUtf8 add option to convert TIS620 string back to UTF-8 before return a string
func OptTis620ToUtf8() OptEinsteinLogger {
	return func(cfg *einsteinLoggerOpts) {
		cfg.tis620ToUtf8 = true
	}
}

// Logger ...
type Logger struct {
	Status                bool
	MW                    io.Writer
	LogPath               string
	LogFileNamePrefix     string
	LogFileNameDateSuffix bool
	LogFile               *os.File
	LogToStdout           bool
	DataFields            EFields
	Opt                   []OptEinsteinLogger
	LastLogTime           time.Time
	LastLogFileOpenTime   time.Time
	LogfileCloseTimeout   int // in second
	LogLevel              uint32
	LogFormat             uint32
	logrusLogger          *logrus.Logger
}

const (
	// LoglvlPanic ...
	LoglvlPanic uint32 = iota
	// LoglvlFatal ...
	LoglvlFatal
	// LoglvlError ...
	LoglvlError
	// LoglvlWarn ...
	LoglvlWarn
	// LoglvlInfo ...
	LoglvlInfo
	// LoglvlDebug ...
	LoglvlDebug
)
const (
	// LogFmtJSON ...
	LogFmtJSON uint32 = iota
	// LogFmtText ...
	LogFmtText
)

func einsteinLogTextFormatter() *prefixed.TextFormatter {
	textFormatter := &prefixed.TextFormatter{}
	textFormatter.ForceFormatting = true
	// textFormatter.ForceColors = true
	textFormatter.FullTimestamp = true
	textFormatter.TimestampFormat = "2006/01/02_15:04:05.000"
	return textFormatter
}

// SetLogLevel ...
func (esLogger *Logger) SetLogLevel(level uint32) error {
	var err error
	if level == LoglvlPanic ||
		level == LoglvlFatal ||
		level == LoglvlError ||
		level == LoglvlWarn ||
		level == LoglvlInfo ||
		level == LoglvlDebug {
		esLogger.LogLevel = level
		esLogger.logrusLogger.Level = logrus.Level(esLogger.LogLevel)
		err = nil
	} else {
		err = errors.New("Log level invlid")
	}
	return err
}
func (esLogger *Logger) isLogfileOpen() bool {
	if esLogger.LogFile == nil || esLogger.MW == nil {
		return false
	}
	return true
}
func (esLogger *Logger) logFileOpen() {
	//create your file with desired read/write permissions
	var err error
	logFilePath := ""
	if esLogger.LogFileNameDateSuffix {
		logFilePath = fmt.Sprintf("%s/%s_%s.log", esLogger.LogPath, esLogger.LogFileNamePrefix, time.Now().Format("20060102"))
	} else {
		logFilePath = fmt.Sprintf("%s/%s.log", esLogger.LogPath, esLogger.LogFileNamePrefix)
	}
	esLogger.LogFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		log.Println(err)
		esLogger.MW = io.MultiWriter(os.Stdout)
	} else {
		//defer to close when you're done with it, not because you think it's idiomatic!
		// defer esLogger.LogFile.Close()
		if esLogger.LogToStdout {
			esLogger.MW = io.MultiWriter(os.Stdout, esLogger.LogFile)
		} else {
			esLogger.MW = io.MultiWriter(esLogger.LogFile)
		}
	}

	// SetOutput Can be any io.Writer
	esLogger.logrusLogger.SetOutput(esLogger.MW)
	esLogger.LastLogTime = time.Now()
	esLogger.LastLogFileOpenTime = time.Now()
	fmt.Printf("FLogger LOGFILE OPENED Path %s\r\n", logFilePath)
}

// LogFileClose ... Clese log file fd
func (esLogger *Logger) LogFileClose() {
	if esLogger.isLogfileOpen() == true {
		// close
		if err := esLogger.LogFile.Close(); err != nil {
			log.Println(err)
		}
		esLogger.LogFile = nil
		esLogger.MW = nil
		esLogger.logrusLogger.SetOutput(nil)
	}
}

// Go routine for close file after no log for few second
func (esLogger *Logger) logFileCloseTimer() {
	forceCheckCloseFileTimer := time.Now()
	for {
		time.Sleep(time.Second * 10)
		if esLogger.isLogfileOpen() {
			elapsed := int(time.Since(esLogger.LastLogTime) / time.Second)
			if elapsed > esLogger.LogfileCloseTimeout { // check timeout sec
				fmt.Printf("FLogger LOGFILE TIMEOUT %v sec, CLOSE FILE\r\n", elapsed)
				esLogger.LogFileClose()
			}
		}

		if esLogger.isLogfileOpen() {
			forceCheckElapsed := int(time.Since(forceCheckCloseFileTimer) / time.Minute)
			if forceCheckElapsed >= 10 { // min
				forceCheckCloseFileTimer = time.Now()
				if esLogger.LastLogFileOpenTime.YearDay() != time.Now().YearDay() {
					fmt.Printf("FLogger LOGFILE Force close 10 min\r\n")
					esLogger.LogFileClose()
				}
			}
		}
	}
}

///////////////////////////////////////////////////////

// Log if you need to call with Options
func (esLogger *Logger) Log(level uint32, mainMsg, extraMsg string, opt ...OptEinsteinLogger) {
	if !esLogger.isLogfileOpen() {
		esLogger.logFileOpen()
	}

	cfg := einsteinLoggerOpts{}
	for _, o := range opt {
		o(&cfg)
	}
	if cfg.tis620ToUtf8 {

	}
	// Add extraMsg as one of EFields
	if len(extraMsg) > 0 {
		esLogger.DataFields["minor_msg"] = extraMsg
	}

	switch level {
	case LoglvlPanic:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Panicln(mainMsg)
	case LoglvlFatal:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Fatalln(mainMsg)
	case LoglvlError:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Errorln(mainMsg)
	case LoglvlWarn:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Warnln(mainMsg)
	case LoglvlInfo:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Infoln(mainMsg)
	case LoglvlDebug:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Debugln(mainMsg)
	default:
		esLogger.logrusLogger.WithFields(logrus.Fields(esLogger.DataFields)).Debugln(mainMsg)
	}
	// Stamp LastLogTime
	esLogger.LastLogTime = time.Now()

	// clear DataFields
	esLogger.DataFields = map[string]interface{}{}
	esLogger.Opt = nil
}

///////////////////////////////////////////////////////

// Panic ...
func (esLogger *Logger) Panic(args ...interface{}) {

	esLogger.Log(LoglvlPanic, fmt.Sprint(args...), "")
}

// Fatal ...
func (esLogger *Logger) Fatal(args ...interface{}) {

	esLogger.Log(LoglvlFatal, fmt.Sprint(args...), "")
}

// Error ...
func (esLogger *Logger) Error(args ...interface{}) {

	esLogger.Log(LoglvlError, fmt.Sprint(args...), "")
}

// Warn ...
func (esLogger *Logger) Warn(args ...interface{}) {

	esLogger.Log(LoglvlWarn, fmt.Sprint(args...), "")
}

// Info ...
func (esLogger *Logger) Info(args ...interface{}) {

	esLogger.Log(LoglvlInfo, fmt.Sprint(args...), "")
}

// Debug ...
func (esLogger *Logger) Debug(args ...interface{}) {

	esLogger.Log(LoglvlDebug, fmt.Sprint(args...), "")
}

///////////////////////////////////////////////////////

// Errorf ...
func (esLogger *Logger) Errorf(format string, args ...interface{}) {
	esLogger.Error(fmt.Sprintf(format, args...), "")
}

// Warnf ...
func (esLogger *Logger) Warnf(format string, args ...interface{}) {
	esLogger.Warn(fmt.Sprintf(format, args...), "")
}

// Infof ...
func (esLogger *Logger) Infof(format string, args ...interface{}) {
	esLogger.Info(fmt.Sprintf(format, args...), "")
}

// Debugf ...
func (esLogger *Logger) Debugf(format string, args ...interface{}) {
	esLogger.Debug(fmt.Sprintf(format, args...), "")
}

///////////////////////////////////////////////////////

// Errorln ...
func (esLogger *Logger) Errorln(args ...interface{}) {
	esLogger.Error(fmt.Sprintln(args...), "")
}

// Warnln ...
func (esLogger *Logger) Warnln(args ...interface{}) {
	esLogger.Warn(fmt.Sprintln(args...), "")
}

// Infoln ...
func (esLogger *Logger) Infoln(args ...interface{}) {
	esLogger.Info(fmt.Sprintln(args...), "")
}

// Debugln ...
func (esLogger *Logger) Debugln(args ...interface{}) {
	esLogger.Debug(fmt.Sprintln(args...), "")
}

// WithFields Add a map of fields to the Entry.
func (esLogger *Logger) WithFields(fields EFields) *Logger {
	esLogger.DataFields = make(EFields, len(fields))
	esLogger.DataFields = fields
	return esLogger
}

// Options Add options to the Entry.
func (esLogger *Logger) Options(opt ...OptEinsteinLogger) *Logger {
	esLogger.Opt = opt
	return esLogger
}

// Init ...
func (esLogger *Logger) Init(logLevel, logFormat uint32, filenamePrefix string, withDateSuffix, toStdout bool, logfileCloseTimeout int) {

	// Make log DIR
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Println("Get Running path error")
		esLogger.LogPath = "~/log"
	} else {
		esLogger.LogPath = fmt.Sprintf("%s/log", dir)
	}
	if len(filenamePrefix) == 0 {
		esLogger.LogFileNamePrefix = "einstein"
	} else {
		esLogger.LogFileNamePrefix = filenamePrefix
	}
	esLogger.LogFileNameDateSuffix = withDateSuffix
	esLogger.LogToStdout = toStdout
	os.MkdirAll(esLogger.LogPath, os.ModePerm)

	esLogger.LogLevel = logLevel
	esLogger.LogFormat = logFormat
	esLogger.LogfileCloseTimeout = logfileCloseTimeout // sec

	esLogger.logrusLogger = logrus.New()
	if esLogger.LogFormat == LogFmtText {
		esLogger.logrusLogger.Formatter = einsteinLogTextFormatter()
	} else if esLogger.LogFormat == LogFmtJSON {
		esLogger.logrusLogger.Formatter = &logrus.JSONFormatter{}
	} else {
		fmt.Println("Log format invalid!!!")
	}

	esLogger.logrusLogger.Level = logrus.Level(esLogger.LogLevel)

	esLogger.logFileOpen()

	// go routine rotation

	// go routine close file timer
	go esLogger.logFileCloseTimer()
}
