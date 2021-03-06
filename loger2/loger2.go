package loger2

import (
    "fmt"
    "log"
    "os"
    "path"
    "runtime"
    "sync/atomic"
    "sync"
    "time"
)

// LogLevel is the log level type.
type LogLevel int

const (
    // DEBUG represents debug log level.
    DEBUG LogLevel = iota
    // INFO represents info log level.
    INFO
    // WARN represents warn log level.
    WARN
    // ERROR represents error log level.
    ERROR
    // FATAL represents fatal log level.
    FATAL
)

var (
    mutex	   sync.Mutex
    started        map[string]int32
    loggerInstance map[string]*Logger
    tagName        = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
    }
)

const DEFAULT_LOGER_NAME="default"

func Init() {
    Init2(1)
}

func Start(decorators ...func(*Logger) *Logger) *Logger {
    return Start2(DEFAULT_LOGER_NAME, decorators...)
}

func (l *Logger) Stop() {
    l.Stop2(DEFAULT_LOGER_NAME)
}

func Init2(size int32) {
    loggerInstance = make(map[string]*Logger, size)
    started = make(map[string]int32, size)
}

// Start returns a decorated innerLogger.
func Start2(name string, decorators ...func(*Logger) *Logger) *Logger {
    if _, ok := started[name]; ok == false {
//    if atomic.CompareAndSwapInt32(&started, 0, 1) {
	loggerInstance[name] = &Logger{}
	for _, decorator := range decorators {
	    loggerInstance[name] = decorator(loggerInstance[name])
	}
	var logger *log.Logger
	var segment *logSegment
	if loggerInstance[name].logPath != "" {
	    segment = newLogSegment(loggerInstance[name].unit, loggerInstance[name].logPath)
	}
	if segment != nil {
	    logger = log.New(segment, "", log.LstdFlags)
	} else {
	    logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	loggerInstance[name].logger = logger

	//atomic.StoreInt32(&started[name], 1)
	mutex.Lock()
	started[name] = 1
	mutex.Unlock()
	return loggerInstance[name]
    }
    panic("Start() already called")
}

// Stop stops the logger.
func (l *Logger) Stop2(name string) {
    if atomic.CompareAndSwapInt32(&l.stopped, 0, 1) {
	if l.printStack {
	    traceInfo := make([]byte, 1<<16)
	    n := runtime.Stack(traceInfo, true)
	    l.logger.Printf("%s", traceInfo[:n])
	    if l.isStdout {
		log.Printf("%s", traceInfo[:n])
	    }
	}
	if l.segment != nil {
	    l.segment.Close()
	}
	l.segment = nil
	l.logger = nil
//	atomic.StoreInt32(&started, 0)
//	atomic.StoreInt32(&started[name], 0)
	mutex.Lock()
	delete(started, name)
	mutex.Unlock()
    }
}

// logSegment implements io.Writer
type logSegment struct {
    unit         time.Duration
    logPath      string
    logFile      *os.File
    timeToCreate <-chan time.Time
}

const (
    DurationSecond time.Duration    = time.Second
    DurationMinute		    = time.Minute
    DurationHour		    = time.Hour
    DurationDay			    = 24 * DurationHour
)

func newLogSegment(unit time.Duration, logPath string) *logSegment {
    now := time.Now()
    if logPath != "" {
	err := os.MkdirAll(logPath, os.ModePerm)
	if err != nil {
	    fmt.Fprintln(os.Stderr, err)
	    return nil
	}
	name := getLogFileName(time.Now())
	logFile, err := os.OpenFile(path.Join(logPath, name), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
	    if os.IsNotExist(err) {
		logFile, err = os.Create(path.Join(logPath, name))
		if err != nil {
		    fmt.Fprintln(os.Stderr, err)
		    return nil
		}
	    } else {
		fmt.Fprintln(os.Stderr, err)
		return nil
	    }
	}
	next := now.Truncate(unit).Add(unit)
	var timeToCreate <-chan time.Time
	if unit == 24*time.Hour || unit == time.Hour || unit == time.Minute {
	    timeToCreate = time.After(next.Sub(time.Now()))
	}
	return &logSegment{
	    unit:         unit,
	    logPath:      logPath,
	    logFile:      logFile,
	    timeToCreate: timeToCreate,
	}
    }
    return nil
}

func (ls *logSegment) Write(p []byte) (n int, err error) {
    if ls.timeToCreate != nil && ls.logFile != os.Stdout && ls.logFile != os.Stderr {
	select {
	case current := <-ls.timeToCreate:
	    ls.logFile.Close()
	    ls.logFile = nil
	    name := getLogFileName(current)
	    ls.logFile, err = os.Create(path.Join(ls.logPath, name))
	    if err != nil {
		// log into stderr if we can't create new file
		fmt.Fprintln(os.Stderr, err)
		ls.logFile = os.Stderr
	    } else {
		next := current.Truncate(ls.unit).Add(ls.unit)
		ls.timeToCreate = time.After(next.Sub(time.Now()))
	    }
	default:
	    // do nothing
	}
    }
    return ls.logFile.Write(p)
}

func (ls *logSegment) Close() {
    ls.logFile.Close()
}

func getLogFileName(t time.Time) string {
    proc := path.Base(os.Args[0])
    now := time.Now()
    year := now.Year()
    month := now.Month()
    day := now.Day()
    hour := now.Hour()
    minute := now.Minute()
    pid := os.Getpid()
    return fmt.Sprintf("%s.%04d-%02d-%02d-%02d-%02d.%d.log",
    proc, year, month, day, hour, minute, pid)
}

// for color formatting
const (
    colorBlack int = iota + 30
    colorRed
    colorGreen
    colorYellow
    colorBlue
    colorMagenta
    colorCyan
    colorWhite
)

func colorSeq(color int) string {
    return fmt.Sprintf("\033[%dm", color)
}

func colorSeqBold(color int) string {
    return fmt.Sprintf("\033[%d;1m", color)
}

func colorOpen(level LogLevel) string {
    switch level {
    case DEBUG:
	return colorSeq(colorWhite)
    case INFO:
	return colorSeqBold(colorWhite)
    case WARN:
	return colorSeqBold(colorYellow)
    case ERROR:
	return colorSeqBold(colorMagenta)
    case FATAL:
	return colorSeqBold(colorRed)
    default:
	return colorSeq(colorWhite)
    }
}

func colorClose() string {
    return "\033[0m"
}

// Logger is the logger type.
type Logger struct {
    logger     *log.Logger
    level      LogLevel
    segment    *logSegment
    stopped    int32
    logPath    string
    unit       time.Duration
    isStdout   bool
    printStack bool
}

func (l Logger) doPrintf(level LogLevel, format string, v ...interface{}) {
    if l.logger == nil {
	return
    }
    if level >= l.level {
	funcName, fileName, lineNum := getRuntimeInfo()
	format = fmt.Sprintf("%5s [%s] (%s:%d) - %s", tagName[level], path.Base(funcName), path.Base(fileName), lineNum, format)
	if l.isStdout {
	    format2 := fmt.Sprintf("%s %s %s", colorOpen(level), format, colorClose())
	    log.Printf(format2, v...)
	}
	l.logger.Printf(format, v...)
	if level == FATAL {
	    os.Exit(1)
	}
    }
}

func (l Logger) doPrintln(level LogLevel, v ...interface{}) {
    if l.logger == nil {
	return
    }
    if level >= l.level {
	funcName, fileName, lineNum := getRuntimeInfo()
	prefix := fmt.Sprintf("%5s [%s] (%s:%d) - ", tagName[level], path.Base(funcName), path.Base(fileName), lineNum)
	value := fmt.Sprintf("%s%s", prefix, fmt.Sprintln(v...))
	if l.isStdout {
	    value2 := fmt.Sprintf("%s %s %s", colorOpen(level), value, colorClose())
	    log.Print(value2)
	}
	l.logger.Print(value)
	if level == FATAL {
	    os.Exit(1)
	}
    }
}

func getRuntimeInfo() (string, string, int) {
    pc, fn, ln, ok := runtime.Caller(3) // 3 steps up the stack frame
    if !ok {
	fn = "???"
	ln = 0
    }
    function := "???"
    caller := runtime.FuncForPC(pc)
    if caller != nil {
	function = caller.Name()
    }
    return function, fn, ln
}

// DebugLevel sets log level to debug.
func DebugLevel(l *Logger) *Logger {
    l.level = DEBUG
    return l
}

// InfoLevel sets log level to info.
func InfoLevel(l *Logger) *Logger {
    l.level = INFO
    return l
}

// WarnLevel sets log level to warn.
func WarnLevel(l *Logger) *Logger {
    l.level = WARN
    return l
}

// ErrorLevel sets log level to error.
func ErrorLevel(l *Logger) *Logger {
    l.level = ERROR
    return l
}

// FatalLevel sets log level to fatal.
func FatalLevel(l *Logger) *Logger {
    l.level = FATAL
    return l
}

// LogFilePath returns a function to set the log file path.
func LogFilePath(p string) func(*Logger) *Logger {
    return func(l *Logger) *Logger {
	l.logPath = p
	return l
    }
}

// EveryHour sets new log file created every hour.
func EveryHour(l *Logger) *Logger {
    l.unit = DurationHour
    return l
}

// EveryMinute sets new log file created every minute.
func EveryMinute(l *Logger) *Logger {
    l.unit = DurationMinute
    return l
}

func EveryDay(l *Logger) *Logger {
    l.unit = DurationDay
    return l
}

// AlsoStdout sets log also output to stdio.
func AlsoStdout(l *Logger) *Logger {
    l.isStdout = true
    return l
}

// PrintStack sets log output the stack trace info.
func PrintStack(l *Logger) *Logger {
    l.printStack = true
    return l
}

// Debugf prints formatted debug log.
func Debugf2(name string, format string, v ...interface{}) {
    loggerInstance[name].doPrintf(DEBUG, format, v...)
}

// Infof prints formatted info log.
func Infof2(name string, format string, v ...interface{}) {
    loggerInstance[name].doPrintf(INFO, format, v...)
}

// Warnf prints formatted warn log.
func Warnf2(name string, format string, v ...interface{}) {
    loggerInstance[name].doPrintf(WARN, format, v...)
}

// Errorf prints formatted error log.
func Errorf2(name string, format string, v ...interface{}) {
    loggerInstance[name].doPrintf(ERROR, format, v...)
}

// Fatalf prints formatted fatal log and exits.
func Fatalf2(name string, format string, v ...interface{}) {
    loggerInstance[name].doPrintf(FATAL, format, v...)
    os.Exit(1)
}

// Debugln prints debug log.
func Debugln2(name string, v ...interface{}) {
    loggerInstance[name].doPrintln(DEBUG, v...)
}

// Infoln prints info log.
func Infoln2(name string, v ...interface{}) {
    loggerInstance[name].doPrintln(INFO, v...)
}

// Warnln prints warn log.
func Warnln2(name string, v ...interface{}) {
    loggerInstance[name].doPrintln(WARN, v...)
}

// Errorln prints error log.
func Errorln2(name string, v ...interface{}) {
    loggerInstance[name].doPrintln(ERROR, v...)
}

// Fatalln prints fatal log and exits.
func Fatalln2(name string, v ...interface{}) {
    loggerInstance[name].doPrintln(FATAL, v...)
    os.Exit(1)
}

/////////////
// Debugf prints formatted debug log.
func Debugf(format string, v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintf(DEBUG, format, v...)
}

// Infof prints formatted info log.
func Infof(format string, v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintf(INFO, format, v...)
}

// Warnf prints formatted warn log.
func Warnf(format string, v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintf(WARN, format, v...)
}

// Errorf prints formatted error log.
func Errorf(format string, v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintf(ERROR, format, v...)
}

// Fatalf prints formatted fatal log and exits.
func Fatalf(format string, v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintf(FATAL, format, v...)
    os.Exit(1)
}

// Debugln prints debug log.
func Debugln(v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintln(DEBUG, v...)
}

// Infoln prints info log.
func Infoln(v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintln(INFO, v...)
}

// Warnln prints warn log.
func Warnln(v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintln(WARN, v...)
}

// Errorln prints error log.
func Errorln(v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintln(ERROR, v...)
}

// Fatalln prints fatal log and exits.
func Fatalln(v ...interface{}) {
    loggerInstance[DEFAULT_LOGER_NAME].doPrintln(FATAL, v...)
    os.Exit(1)
}
