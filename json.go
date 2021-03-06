package log

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// DefaultLogger is the global logger.
var DefaultLogger = Logger{
	Level:      DebugLevel,
	Caller:     0,
	TimeField:  "",
	TimeFormat: "",
	Timestamp:  false,
	HostField:  "",
	Writer:     os.Stderr,
}

// A Logger represents an active logging object that generates lines of JSON output to an io.Writer.
type Logger struct {
	// Level defines log levels.
	Level Level

	// Timestamp determines if time is formatted as an UNIX timestamp as integer.
	// If set, the value of TimeField and TimeFormat will be ignored.
	Timestamp bool

	// Caller determines if adds the file:line of the "caller" key.
	Caller int

	// TimeField defines the time filed name in output.  It uses "time" in if empty.
	TimeField string

	// TimeFormat specifies the time format in output. It uses time.RFC3389 in if empty.
	TimeFormat string

	// HostField specifies the key for hostname in output if not empty
	HostField string

	// Writer specifies the writer of output. It uses os.Stderr in if empty.
	Writer io.Writer
}

// Event represents a log event. It is instanced by one of the level method of Logger and finalized by the Msg or Msgf method.
type Event struct {
	buf   []byte
	w     io.Writer
	stack bool
	exit  bool
}

// Debug starts a new message with debug level.
func Debug() (e *Event) {
	e = DefaultLogger.header(DebugLevel)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	return
}

// Info starts a new message with info level.
func Info() (e *Event) {
	e = DefaultLogger.header(InfoLevel)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	return
}

// Warn starts a new message with warning level.
func Warn() (e *Event) {
	e = DefaultLogger.header(WarnLevel)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	return
}

// Error starts a new message with error level.
func Error() (e *Event) {
	e = DefaultLogger.header(ErrorLevel)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	return
}

// Fatal starts a new message with fatal level.
func Fatal() (e *Event) {
	e = DefaultLogger.header(FatalLevel)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	return
}

// Print sends a log event using debug level and no extra field. Arguments are handled in the manner of fmt.Print.
func Print(v ...interface{}) {
	e := DefaultLogger.header(DefaultLogger.Level)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	e.print(v...)
}

// Printf sends a log event using debug level and no extra field. Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...interface{}) {
	e := DefaultLogger.header(DefaultLogger.Level)
	if e != nil && DefaultLogger.Caller > 0 {
		e.caller(runtime.Caller(DefaultLogger.Caller))
	}
	e.Msgf(format, v...)
}

// Debug starts a new message with debug level.
func (l *Logger) Debug() (e *Event) {
	e = l.header(DebugLevel)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// Info starts a new message with info level.
func (l *Logger) Info() (e *Event) {
	e = l.header(InfoLevel)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// Warn starts a new message with warning level.
func (l *Logger) Warn() (e *Event) {
	e = l.header(WarnLevel)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// Error starts a new message with error level.
func (l *Logger) Error() (e *Event) {
	e = l.header(ErrorLevel)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// Fatal starts a new message with fatal level.
func (l *Logger) Fatal() (e *Event) {
	e = l.header(FatalLevel)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// WithLevel starts a new message with level.
func (l *Logger) WithLevel(level Level) (e *Event) {
	e = l.header(level)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	return
}

// SetLevel changes logger default level.
func (l *Logger) SetLevel(level Level) {
	atomic.StoreUint32((*uint32)(&l.Level), uint32(level))
	return
}

// Print sends a log event using debug level and no extra field. Arguments are handled in the manner of fmt.Print.
func (l *Logger) Print(v ...interface{}) {
	e := l.header(l.Level)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	e.print(v...)
}

// Printf sends a log event using debug level and no extra field. Arguments are handled in the manner of fmt.Printf.
func (l *Logger) Printf(format string, v ...interface{}) {
	e := l.header(l.Level)
	if e != nil && l.Caller > 0 {
		e.caller(runtime.Caller(l.Caller))
	}
	e.Msgf(format, v...)
}

var epool = sync.Pool{
	New: func() interface{} {
		return &Event{
			buf: make([]byte, 0, 500),
		}
	},
}

const smallsString = "00010203040506070809" +
	"10111213141516171819" +
	"20212223242526272829" +
	"30313233343536373839" +
	"40414243444546474849" +
	"50515253545556575859" +
	"60616263646566676869" +
	"70717273747576777879" +
	"80818283848586878889" +
	"90919293949596979899"

var timeNow = time.Now

var hostname, _ = os.Hostname()

func (l *Logger) header(level Level) *Event {
	if uint32(level) < atomic.LoadUint32((*uint32)(&l.Level)) {
		return nil
	}
	e := epool.Get().(*Event)
	e.buf = e.buf[:0]
	e.stack = level == FatalLevel
	e.exit = level == FatalLevel
	if l.Writer != nil {
		e.w = l.Writer
	} else {
		e.w = os.Stderr
	}
	// time
	if l.Timestamp {
		e.buf = append(e.buf, "{\"time\":0465408000000"...)
		sec, nsec := walltime()
		// milli seconds
		a := int64(nsec) / 1000000
		is := a % 100 * 2
		e.buf[20] = smallsString[is+1]
		e.buf[19] = smallsString[is]
		e.buf[18] = byte('0' + a/100)
		// seconds
		is = sec % 100 * 2
		sec /= 100
		e.buf[17] = smallsString[is+1]
		e.buf[16] = smallsString[is]
		is = sec % 100 * 2
		sec /= 100
		e.buf[15] = smallsString[is+1]
		e.buf[14] = smallsString[is]
		is = sec % 100 * 2
		sec /= 100
		e.buf[13] = smallsString[is+1]
		e.buf[12] = smallsString[is]
		is = sec % 100 * 2
		sec /= 100
		e.buf[11] = smallsString[is+1]
		e.buf[10] = smallsString[is]
		is = sec % 100 * 2
		e.buf[9] = smallsString[is+1]
		e.buf[8] = smallsString[is]
	} else {
		if l.TimeField == "" {
			e.buf = append(e.buf, "{\"time\":"...)
		} else {
			e.buf = append(e.buf, '{', '"')
			e.buf = append(e.buf, l.TimeField...)
			e.buf = append(e.buf, '"', ':')
		}
		if l.TimeFormat == "" {
			e.time(walltime())
		} else {
			e.buf = append(e.buf, '"')
			e.buf = timeNow().AppendFormat(e.buf, l.TimeFormat)
			e.buf = append(e.buf, '"')
		}
	}
	// level
	switch level {
	case DebugLevel:
		e.buf = append(e.buf, ",\"level\":\"debug\""...)
	case InfoLevel:
		e.buf = append(e.buf, ",\"level\":\"info\""...)
	case WarnLevel:
		e.buf = append(e.buf, ",\"level\":\"warn\""...)
	case ErrorLevel:
		e.buf = append(e.buf, ",\"level\":\"error\""...)
	case FatalLevel:
		e.buf = append(e.buf, ",\"level\":\"fatal\""...)
	}
	// hostname
	if l.HostField != "" {
		e.buf = append(e.buf, ',', '"')
		e.buf = append(e.buf, l.HostField...)
		e.buf = append(e.buf, '"', ':', '"')
		e.buf = append(e.buf, hostname...)
		e.buf = append(e.buf, '"')
	}
	return e
}

// Time append append t formated as string using time.RFC3339Nano.
func (e *Event) Time(key string, t time.Time) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	e.buf = t.AppendFormat(e.buf, time.RFC3339Nano)
	e.buf = append(e.buf, '"')
	return e
}

// TimeFormat append append t formated as string using timefmt.
func (e *Event) TimeFormat(key string, timefmt string, t time.Time) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	e.buf = t.AppendFormat(e.buf, timefmt)
	e.buf = append(e.buf, '"')
	return e
}

// Bool append append the val as a bool to the event.
func (e *Event) Bool(key string, b bool) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = strconv.AppendBool(e.buf, b)
	return e
}

// Bools adds the field key with val as a []bool to the event.
func (e *Event) Bools(key string, b []bool) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '[')
	for i, a := range b {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		e.buf = strconv.AppendBool(e.buf, a)
	}
	e.buf = append(e.buf, ']')
	return e
}

// Dur adds the field key with duration d to the event.
func (e *Event) Dur(key string, d time.Duration) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, d.String()...)
	e.buf = append(e.buf, '"')
	return e
}

// Durs adds the field key with val as a []time.Duration to the event.
func (e *Event) Durs(key string, d []time.Duration) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '[')
	for i, a := range d {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		e.buf = append(e.buf, '"')
		e.buf = append(e.buf, a.String()...)
		e.buf = append(e.buf, '"')
	}
	e.buf = append(e.buf, ']')
	return e
}

// Err adds the field "error" with serialized err to the event.
func (e *Event) Err(err error) *Event {
	if e == nil {
		return nil
	}
	if err == nil {
		e.buf = append(e.buf, ",\"error\":null"...)
	} else {
		e.buf = append(e.buf, ",\"error\":"...)
		e.string(err.Error())
	}
	return e
}

// Errs adds the field key with errs as an array of serialized errors to the event.
func (e *Event) Errs(key string, errs []error) *Event {
	if e == nil {
		return nil
	}

	e.key(key)
	e.buf = append(e.buf, '[')
	for i, err := range errs {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		if err == nil {
			e.buf = append(e.buf, "null"...)
		} else {
			e.string(err.Error())
		}
	}
	e.buf = append(e.buf, ']')
	return e
}

// Float64 adds the field key with f as a float64 to the event.
func (e *Event) Float64(key string, f float64) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = strconv.AppendFloat(e.buf, f, 'f', -1, 64)
	return e
}

// Floats64 adds the field key with f as a []float64 to the event.
func (e *Event) Floats64(key string, f []float64) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '[')
	for i, a := range f {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		e.buf = strconv.AppendFloat(e.buf, a, 'f', -1, 64)
	}
	e.buf = append(e.buf, ']')
	return e
}

// Floats32 adds the field key with f as a []float32 to the event.
func (e *Event) Floats32(key string, f []float32) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '[')
	for i, a := range f {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		e.buf = strconv.AppendFloat(e.buf, float64(a), 'f', -1, 64)
	}
	e.buf = append(e.buf, ']')
	return e
}

// Int64 adds the field key with i as a int64 to the event.
func (e *Event) Int64(key string, i int64) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = strconv.AppendInt(e.buf, i, 10)
	return e
}

// Uint64 adds the field key with i as a uint64 to the event.
func (e *Event) Uint64(key string, i uint64) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = strconv.AppendUint(e.buf, i, 10)
	return e
}

// Float32 adds the field key with f as a float32 to the event.
func (e *Event) Float32(key string, f float32) *Event {
	return e.Float64(key, float64(f))
}

// Int adds the field key with i as a int to the event.
func (e *Event) Int(key string, i int) *Event {
	return e.Int64(key, int64(i))
}

// Int32 adds the field key with i as a int32 to the event.
func (e *Event) Int32(key string, i int32) *Event {
	return e.Int64(key, int64(i))
}

// Int16 adds the field key with i as a int16 to the event.
func (e *Event) Int16(key string, i int16) *Event {
	return e.Int64(key, int64(i))
}

// Int8 adds the field key with i as a int8 to the event.
func (e *Event) Int8(key string, i int8) *Event {
	return e.Int64(key, int64(i))
}

// Uint32 adds the field key with i as a uint32 to the event.
func (e *Event) Uint32(key string, i uint32) *Event {
	return e.Uint64(key, uint64(i))
}

// Uint16 adds the field key with i as a uint16 to the event.
func (e *Event) Uint16(key string, i uint16) *Event {
	return e.Uint64(key, uint64(i))
}

// Uint8 adds the field key with i as a uint8 to the event.
func (e *Event) Uint8(key string, i uint8) *Event {
	return e.Uint64(key, uint64(i))
}

// RawJSON adds already encoded JSON to the log line under key.
func (e *Event) RawJSON(key string, b []byte) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, b...)
	return e
}

// Str adds the field key with val as a string to the event.
func (e *Event) Str(key string, val string) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.string(val)
	return e
}

// Strs adds the field key with vals as a []string to the event.
func (e *Event) Strs(key string, vals []string) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '[')
	for i, val := range vals {
		if i != 0 {
			e.buf = append(e.buf, ',')
		}
		e.string(val)
	}
	e.buf = append(e.buf, ']')
	return e
}

// Bytes adds the field key with val as a string to the event.
func (e *Event) Bytes(key string, val []byte) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.bytes(val)
	return e
}

var hex = "0123456789abcdef"

// Hex adds the field key with val as a hex string to the event.
func (e *Event) Hex(key string, val []byte) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	for _, v := range val {
		e.buf = append(e.buf, hex[v>>4], hex[v&0x0f])
	}
	e.buf = append(e.buf, '"')
	return e
}

// IPAddr adds IPv4 or IPv6 Address to the event
func (e *Event) IPAddr(key string, ip net.IP) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	if ip4 := ip.To4(); ip4 != nil {
		e.buf = strconv.AppendInt(e.buf, int64(ip4[0]), 10)
		e.buf = append(e.buf, '.')
		e.buf = strconv.AppendInt(e.buf, int64(ip4[1]), 10)
		e.buf = append(e.buf, '.')
		e.buf = strconv.AppendInt(e.buf, int64(ip4[2]), 10)
		e.buf = append(e.buf, '.')
		e.buf = strconv.AppendInt(e.buf, int64(ip4[3]), 10)
	} else {
		e.buf = append(e.buf, ip.String()...)
	}
	e.buf = append(e.buf, '"')
	return e
}

// IPPrefix adds IPv4 or IPv6 Prefix (address and mask) to the event
func (e *Event) IPPrefix(key string, pfx net.IPNet) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, pfx.String()...)
	e.buf = append(e.buf, '"')
	return e
}

// MACAddr adds MAC address to the event
func (e *Event) MACAddr(key string, ha net.HardwareAddr) *Event {
	if e == nil {
		return nil
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	for i, c := range ha {
		if i > 0 {
			e.buf = append(e.buf, ':')
		}
		e.buf = append(e.buf, hex[c>>4])
		e.buf = append(e.buf, hex[c&0xF])
	}
	e.buf = append(e.buf, '"')
	return e
}

// TimeDiff adds the field key with positive duration between time t and start.
// If time t is not greater than start, duration will be 0.
// Duration format follows the same principle as Dur().
func (e *Event) TimeDiff(key string, t time.Time, start time.Time) *Event {
	if e == nil {
		return e
	}
	var d time.Duration
	if t.After(start) {
		d = t.Sub(start)
	}
	e.key(key)
	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, d.String()...)
	e.buf = append(e.buf, '"')
	return e
}

// Caller adds the file:line of the "caller" key.
func (e *Event) Caller() *Event {
	if e == nil {
		return nil
	}
	e.caller(runtime.Caller(DefaultLogger.Caller))
	return e
}

// Stack enables stack trace printing for the error passed to Err().
func (e *Event) Stack() *Event {
	if e == nil {
		return nil
	}
	e.stack = true
	return e
}

// Enabled return false if the event is going to be filtered out by log level.
func (e *Event) Enabled() bool {
	return e != nil
}

// Discard disables the event so Msg(f) won't print it.
func (e *Event) Discard() *Event {
	if e == nil {
		return e
	}
	if cap(e.buf) <= bbcap {
		epool.Put(e)
	}
	return nil
}

var osExit = os.Exit

// Msg sends the event with msg added as the message field if not empty.
func (e *Event) Msg(msg string) {
	if e == nil {
		return
	}
	if msg != "" {
		e.buf = append(e.buf, ",\"message\":"...)
		e.string(msg)
	}
	e.buf = append(e.buf, '}', '\n')
	e.w.Write(e.buf)
	if e.stack {
		e.w.Write(stacks(false))
		e.w.Write(stacks(true))
	}
	if e.exit {
		osExit(255)
	}
	if cap(e.buf) <= bbcap {
		epool.Put(e)
	}
}

func (e *Event) key(key string) {
	e.buf = append(e.buf, ',', '"')
	e.buf = append(e.buf, key...)
	e.buf = append(e.buf, '"', ':')
}

func (e *Event) caller(_ uintptr, file string, line int, _ bool) {
	if i := strings.LastIndex(file, "/"); i >= 0 {
		file = file[i+1:]
	}
	e.buf = append(e.buf, ",\"caller\":\""...)
	e.buf = append(e.buf, file...)
	e.buf = append(e.buf, ':')
	e.buf = strconv.AppendInt(e.buf, int64(line), 10)
	e.buf = append(e.buf, '"')
}

const timebuf = "\"2006-01-02T15:04:05.999Z\""

func (e *Event) time(sec int64, nsec int32) {
	n := len(e.buf)
	if n+len(timebuf) < cap(e.buf) {
		e.buf = e.buf[:n+len(timebuf)]
	} else {
		e.buf = append(e.buf, timebuf...)
	}
	var a, b int
	// milli second
	e.buf[n+25] = '"'
	e.buf[n+24] = 'Z'
	a = int(nsec) / 1000000
	b = a / 10
	e.buf[n+23] = byte('0' + a - 10*b)
	a = b
	b = a / 10
	e.buf[n+22] = byte('0' + a - 10*b)
	e.buf[n+21] = byte('0' + b)
	e.buf[n+20] = '.'
	// date time
	sec += 9223372028715321600 // unixToInternal + internalToAbsolute
	year, month, day, _ := absDate(uint64(sec), true)
	hour, minute, second := absClock(uint64(sec))
	// year
	a = year
	b = a / 10
	e.buf[n+4] = byte('0' + a - 10*b)
	a = b
	b = a / 10
	e.buf[n+3] = byte('0' + a - 10*b)
	a = b
	b = a / 10
	e.buf[n+2] = byte('0' + a - 10*b)
	e.buf[n+1] = byte('0' + b)
	e.buf[n] = '"'
	// month
	a = int(month)
	b = a / 10
	e.buf[n+7] = byte('0' + a - 10*b)
	e.buf[n+6] = byte('0' + b)
	e.buf[n+5] = '-'
	// day
	a = day
	b = a / 10
	e.buf[n+10] = byte('0' + a - 10*b)
	e.buf[n+9] = byte('0' + b)
	e.buf[n+8] = '-'
	// hour
	a = hour
	b = a / 10
	e.buf[n+13] = byte('0' + a - 10*b)
	e.buf[n+12] = byte('0' + b)
	e.buf[n+11] = 'T'
	// minute
	a = minute
	b = a / 10
	e.buf[n+16] = byte('0' + a - 10*b)
	e.buf[n+15] = byte('0' + b)
	e.buf[n+14] = ':'
	// second
	a = second
	b = a / 10
	e.buf[n+19] = byte('0' + a - 10*b)
	e.buf[n+18] = byte('0' + b)
	e.buf[n+17] = ':'
}

var escapes = func() (a [256]bool) {
	a['"'] = true
	a['<'] = true
	a['\''] = true
	a['\\'] = true
	a['\b'] = true
	a['\f'] = true
	a['\n'] = true
	a['\r'] = true
	a['\t'] = true
	a[0] = true
	return
}()

func (e *Event) escape(b []byte) {
	e.buf = append(e.buf, '"')
	n := len(b)
	j := 0
	if n > 0 {
		// Hint the compiler to remove bounds checks in the loop below.
		_ = b[n-1]
	}
	for i := 0; i < n; i++ {
		switch b[i] {
		case '"':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', '"')
			j = i + 1
		case '\\':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', '\\')
			j = i + 1
		case '\n':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'n')
			j = i + 1
		case '\r':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'r')
			j = i + 1
		case '\t':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 't')
			j = i + 1
		case '\f':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'u', '0', '0', '0', 'c')
			j = i + 1
		case '\b':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'u', '0', '0', '0', '8')
			j = i + 1
		case '<':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'u', '0', '0', '3', 'c')
			j = i + 1
		case '\'':
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'u', '0', '0', '2', '7')
			j = i + 1
		case 0:
			e.buf = append(e.buf, b[j:i]...)
			e.buf = append(e.buf, '\\', 'u', '0', '0', '0', '0')
			j = i + 1
		}
	}
	e.buf = append(e.buf, b[j:]...)
	e.buf = append(e.buf, '"')
}

func (e *Event) string(s string) {
	for _, c := range []byte(s) {
		if escapes[c] {
			sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
			b := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: sh.Data, Len: sh.Len, Cap: sh.Len,
			}))
			e.escape(b)
			return
		}
	}

	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, '"')

	return
}

func (e *Event) bytes(b []byte) {
	for _, c := range b {
		if escapes[c] {
			e.escape(b)
			return
		}
	}

	e.buf = append(e.buf, '"')
	e.buf = append(e.buf, b...)
	e.buf = append(e.buf, '"')

	return
}

type bb struct {
	B []byte
}

func (b *bb) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	return len(p), nil
}

func (b *bb) Reset() {
	b.B = b.B[:0]
}

var bbpool = sync.Pool{
	New: func() interface{} {
		return new(bb)
	},
}

const bbcap = 1 << 16

// Interface adds the field key with i marshaled using reflection.
func (e *Event) Interface(key string, i interface{}) *Event {
	if e == nil {
		return nil
	}
	e.key(key)

	b := bbpool.Get().(*bb)
	b.Reset()

	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)

	err := enc.Encode(i)
	if err != nil {
		e.string("marshaling error: " + err.Error())
	} else {
		e.bytes(b.B)
	}

	if cap(b.B) <= bbcap {
		bbpool.Put(b)
	}

	return e
}

// print sends the event with msgs added as the message field if not empty.
func (e *Event) print(v ...interface{}) {
	if e == nil {
		return
	}

	b := bbpool.Get().(*bb)
	b.Reset()

	fmt.Fprint(b, v...)
	e.Msg(*(*string)(unsafe.Pointer(&b.B)))

	if cap(b.B) <= bbcap {
		bbpool.Put(b)
	}
}

// Msgf sends the event with formatted msg added as the message field if not empty.
func (e *Event) Msgf(format string, v ...interface{}) {
	if e == nil {
		return
	}

	b := bbpool.Get().(*bb)
	b.Reset()

	fmt.Fprintf(b, format, v...)
	e.Msg(*(*string)(unsafe.Pointer(&b.B)))

	if cap(b.B) <= bbcap {
		bbpool.Put(b)
	}
}

// stacks is a wrapper for runtime.Stack that attempts to recover the data for all goroutines.
func stacks(all bool) []byte {
	// We don't know how big the traces are, so grow a few times if they don't fit. Start large, though.
	n := 10000
	if all {
		n = 100000
	}
	var trace []byte
	for i := 0; i < 5; i++ {
		trace = make([]byte, n)
		nbytes := runtime.Stack(trace, all)
		if nbytes < len(trace) {
			return trace[:nbytes]
		}
		n *= 2
	}
	return trace
}

//go:noescape
//go:linkname absDate time.absDate
func absDate(abs uint64, full bool) (year int, month time.Month, day int, yday int)

//go:noescape
//go:linkname absClock time.absClock
func absClock(abs uint64) (hour, min, sec int)

// Fastrandn returns a pseudorandom uint32 in [0,n).
//go:noescape
//go:linkname Fastrandn runtime.fastrandn
func Fastrandn(x uint32) uint32
