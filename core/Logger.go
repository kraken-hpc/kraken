/* Logger.go: Implementation of the WriterLogger & ServiceLogger
 *
 * Author: J. Lowell Wofford <lowell@lanl.gov>
 *
 * This software is open source software available under the BSD-3 license.
 * Copyright (c) 2018, Triad National Security, LLC
 * See LICENSE file for details.
 */

package core

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hpc/kraken/lib"
)

///////////////////////
// Auxiliary Objects /
/////////////////////

// create shortcut aliases for log levels
const (
	DDDEBUG  = lib.LLDDDEBUG
	DDEBUG   = lib.LLDDEBUG
	DEBUG    = lib.LLDEBUG
	INFO     = lib.LLINFO
	NOTICE   = lib.LLNOTICE
	WARNING  = lib.LLWARNING
	ERROR    = lib.LLERROR
	CRITICAL = lib.LLCRITICAL
	FATAL    = lib.LLFATAL
	PANIC    = lib.LLPANIC
)

// LoggerEvent is used by ServiceLogger to send log events over channels
type LoggerEvent struct {
	Level   lib.LoggerLevel
	Module  string
	Message string
}

// ServiceLoggerListener receives log events through a channel and passes them to a secondary logger interface
// This can be run as a goroutine
func ServiceLoggerListener(l lib.Logger, c <-chan LoggerEvent) {
	for {
		le := <-c
		l.Log(le.Level, le.Module+":"+le.Message)
	}
}

/////////////////////////
// WriterLogger Object /
///////////////////////

var _ lib.Logger = (*WriterLogger)(nil)

// A WriterLogger writes to any io.Writer interface, e.g. stdout, stderr, or an open file
// NOTE: Does not close the interface
type WriterLogger struct {
	w             io.Writer
	m             string
	lv            lib.LoggerLevel
	DisablePrefix bool
}

// Log submits a Log message with a LoggerLevel
func (l *WriterLogger) Log(lv lib.LoggerLevel, m string) {
	if l.IsEnabledFor(lv) {
		plv := string(lv)
		if int(lv) <= len(lib.LoggerLevels)+1 {
			plv = lib.LoggerLevels[lv]
		}
		var s []string
		if !l.DisablePrefix {
			s = []string{
				time.Now().Format("15:04:05.000"),
				l.m,
				plv,
				strings.TrimSpace(m) + "\n",
			}
		} else {
			s = []string{
				l.m,
				plv,
				strings.TrimSpace(m) + "\n",
			}
		}
		l.w.Write([]byte(strings.Join(s, ":")))
	}
}

// Logf is the same as Log but with sprintf formatting
func (l *WriterLogger) Logf(lv lib.LoggerLevel, f string, v ...interface{}) {
	if l.IsEnabledFor(lv) {
		l.Log(lv, fmt.Sprintf(f, v...))
	}
}

// SetModule sets an identifier string for the component that will use this Logger
func (l *WriterLogger) SetModule(m string) { l.m = m }

// GetModule gets the current module string
func (l *WriterLogger) GetModule() string { return l.m }

// SetLoggerLevel sets the log filtering level
func (l *WriterLogger) SetLoggerLevel(lv lib.LoggerLevel) { l.lv = lv }

// GetLoggerLevel gets the log filtering level
func (l *WriterLogger) GetLoggerLevel() lib.LoggerLevel { return l.lv }

// IsEnabledFor determines if this Logger would send a message at a particular level
func (l *WriterLogger) IsEnabledFor(lv lib.LoggerLevel) (r bool) {
	if lv <= l.lv {
		return true
	}
	return
}

// RegisterWriter sets the writer interface this logger will use
func (l *WriterLogger) RegisterWriter(w io.Writer) {
	l.w = w
}

//////////////////////////
// ServiceLogger Object /
////////////////////////

var _ lib.Logger = (*ServiceLogger)(nil)

// A ServiceLogger is a channel interface for aggregating logs from services running as separate goroutines
type ServiceLogger struct {
	c  chan<- LoggerEvent
	m  string
	lv lib.LoggerLevel
}

func (l *ServiceLogger) Log(lv lib.LoggerLevel, m string) {
	if l.IsEnabledFor(lv) {
		l.c <- LoggerEvent{
			Level:   lv,
			Module:  l.m,
			Message: m,
		}
	}
}
func (l *ServiceLogger) Logf(lv lib.LoggerLevel, f string, v ...interface{}) {
	if l.IsEnabledFor(lv) {
		l.Log(lv, fmt.Sprintf(f, v...))
	}
}
func (l *ServiceLogger) SetModule(m string)                { l.m = m }
func (l *ServiceLogger) GetModule() string                 { return l.m }
func (l *ServiceLogger) SetLoggerLevel(lv lib.LoggerLevel) { l.lv = lv }
func (l *ServiceLogger) GetLoggerLevel() lib.LoggerLevel   { return l.lv }
func (l *ServiceLogger) IsEnabledFor(lv lib.LoggerLevel) (r bool) {
	if lv <= l.lv {
		return true
	}
	return
}

// RegisterChannel sets the chan that the ServiceLogger will send events over
func (l *ServiceLogger) RegisterChannel(c chan<- LoggerEvent) { l.c = c }
