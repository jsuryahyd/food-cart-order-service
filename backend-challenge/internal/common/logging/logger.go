package logging

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger = zap.SugaredLogger

var logger *zap.Logger

// InitLogger initializes the zap logger for production
func InitLogger() error {
	var err error
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err = cfg.Build()
	if err != nil {
		return err
	}
	return nil
}

// GetLogger returns the global zap logger
func GetLogger() *Logger {
	if logger == nil {
		_ = InitLogger()
	}
	return logger.Sugar()
}

// Override gin's default logger for logging api requests, adds latency of each request
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		logArgs := []interface{}{
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"ip", c.ClientIP(),
			"user-agent", c.Request.UserAgent(),
			"latency", latency,
		}
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logArgs = append(logArgs, "gin_error", e)
			}
		}
		GetLogger().Infow("request", logArgs...)
	}
}

// WithContext returns a logger with context fields (for request-scoped logging)
func WithContext(ctx context.Context) *Logger {
	return GetLogger().With("request_id", getRequestID(ctx))
}

// getRequestID extracts a request ID from context if available
func getRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
