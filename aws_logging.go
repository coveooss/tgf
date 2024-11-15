package main

import (
	"github.com/aws/smithy-go/logging"
	"github.com/coveooss/multilogger"
)

type AwsLogger struct {
	*multilogger.Logger
}

func NewAwsLogger(module string, hooks ...*multilogger.Hook) *AwsLogger {
	logger := multilogger.New(module, hooks...)
	return &AwsLogger{logger}
}

func (log *AwsLogger) Logf(classification logging.Classification, format string, v ...interface{}) {
	switch classification {
	case logging.Debug:
		log.Debugf(format, v...)
	case logging.Warn:
		log.Warningf(format, v...)
	}
}
