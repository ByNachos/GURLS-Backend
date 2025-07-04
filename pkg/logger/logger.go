package logger

import (
	"log"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New initializes and returns a new zap.Logger configured for development
// with timestamps in the "Europe/Moscow" timezone.
// It will panic if initialization fails (e.g., timezone cannot be loaded).
func New(env string) *zap.Logger {
	// Load the Moscow timezone. Panic on error as the logger is critical.
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatalf("failed to load Moscow timezone: %v", err)
	}

	// Use the development config for human-readable, colored output.
	encoderCfg := zap.NewDevelopmentEncoderConfig()

	// Customize the time encoder to use Moscow time.
	encoderCfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.In(location).Format("2006-01-02 15:04:05"))
	}
	// Also, customize the log level key to be more readable.
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Now, let's build the logger from more fundamental pieces.
	// This gives us the control to inject our custom encoder config.

	// 1. Create the Encoder using our custom config.
	// NewConsoleEncoder is for human-readable, terminal-friendly output.
	encoder := zapcore.NewConsoleEncoder(encoderCfg)

	// 2. Define where to write logs (in our case, to standard output).
	writeSyncer := zapcore.AddSync(os.Stdout)

	// 3. Define the minimum log level.
	// We'll use DebugLevel for now, which logs everything.
	level := zapcore.DebugLevel

	// 4. Create the Core that combines the encoder, writer, and level.
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 5. Build the final logger, adding extra helpful options.
	//    - AddCaller adds the file:line of the log call (e.g., "main.go:25").
	//    - AddStacktrace automatically records a stack trace for logs at ErrorLevel and above.
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger
}
