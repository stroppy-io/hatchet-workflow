package s3

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getDefaultAwsLogMode(lgr *zap.Logger) aws.ClientLogMode {
	clientLogMode := aws.LogRequest | aws.LogResponse | aws.LogRetries
	if lgr.Level() == zapcore.DebugLevel {
		clientLogMode = clientLogMode |
			aws.LogDeprecatedUsage |
			aws.LogRequestEventMessage |
			aws.LogResponseEventMessage |
			aws.LogSigning |
			aws.LogRequestWithBody |
			aws.LogResponseWithBody
	}
	return clientLogMode
}

func getDefaultAwsLoggerFunc(lgr *zap.Logger) logging.LoggerFunc {
	return func(classification logging.Classification, format string, v ...interface{}) {
		switch classification {
		case logging.Debug:
			lgr.Sugar().Debugf(format, v...)
		case logging.Warn:
			lgr.Sugar().Warnf(format, v...)
		}
	}
}
