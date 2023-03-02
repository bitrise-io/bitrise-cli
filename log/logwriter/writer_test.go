package logwriter_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/bitrise-io/bitrise/log"
	"github.com/bitrise-io/bitrise/log/logwriter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONLog(t *testing.T) {
	tests := []struct {
		name             string
		messages         []string
		expectedMessages []string
	}{
		{
			name:             "Writes messages with Normal log level by default",
			messages:         []string{"Hello Bitrise!"},
			expectedMessages: []string{`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"Hello Bitrise!"}` + "\n"},
		},
		{
			name:             "Detects log level",
			messages:         []string{"\u001B[34;1mLogin to the service\u001B[0m"},
			expectedMessages: []string{`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"info","message":"Login to the service"}` + "\n"},
		},
		{
			name: "Detects a log level in a message stream",
			messages: []string{
				"\u001B[35;1mdetected login method:",
				"- API key",
				"- username\u001B[0m",
			},
			expectedMessages: []string{`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"debug","message":"detected login method:\n- API key\n- username"}` + "\n"},
		},
		{
			name: "Detects multiple messages with log level in the message stream",
			messages: []string{
				"Hello Bitrise!",
				"\u001B[35;1mdetected login method:",
				"- API key",
				"- username\u001B[0m",
				"\u001B[34;1mLogin to the service\u001B[0m",
			},
			expectedMessages: []string{
				`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"Hello Bitrise!"}` + "\n",
				`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"debug","message":"detected login method:\n- API key\n- username"}` + "\n",
				`{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"info","message":"Login to the service"}` + "\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			opts := log.LoggerOpts{
				LoggerType:        log.JSONLogger,
				ConsoleLoggerOpts: log.ConsoleLoggerOpts{},
				DebugLogEnabled:   true,
				Writer:            &buf,
				TimeProvider: func() time.Time {
					return time.Time{}
				},
			}
			logger := log.NewLogger(opts)
			writer := logwriter.NewLogWriter(logger)

			var actualMessages []string
			for _, message := range tt.messages {
				b := []byte(message)

				_, err := writer.Write(b)
				assert.NoError(t, err)

				actualMessage := buf.String()
				buf.Reset()
				if actualMessage != "" {
					actualMessages = append(actualMessages, actualMessage)
				}
			}

			err := writer.Close()
			require.NoError(t, err)

			require.Equal(t, tt.expectedMessages, actualMessages)
		})
	}
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name             string
		messages         []string
		expectedMessages []string
	}{
		{
			name:             "Writes messages without log level as it is",
			messages:         []string{"Hello Bitrise!"},
			expectedMessages: []string{"Hello Bitrise!"},
		},
		{
			name:             "Writes messages with log level as it is",
			messages:         []string{"\u001B[34;1mLogin to the service\u001B[0m"},
			expectedMessages: []string{"\u001B[34;1mLogin to the service\u001B[0m"},
		},
		{
			name: "Detects a message with log level in the message stream",
			messages: []string{
				"\u001B[35;1mdetected login method:",
				"- API key",
				"- username\u001B[0m",
			},
			expectedMessages: []string{"\u001B[35;1mdetected login method:\n- API key\n- username\u001B[0m"},
		},
		{
			name: "Detects multiple messages with log level in the message stream",
			messages: []string{
				"Hello Bitrise!",
				"\u001B[35;1mdetected login method:",
				"- API key",
				"- username\u001B[0m",
				"\u001B[34;1mLogin to the service\u001B[0m",
			},
			expectedMessages: []string{
				"Hello Bitrise!",
				"\u001B[35;1mdetected login method:\n- API key\n- username\u001B[0m",
				"\u001B[34;1mLogin to the service\u001B[0m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			opts := log.LoggerOpts{
				ConsoleLoggerOpts: log.ConsoleLoggerOpts{},
				DebugLogEnabled:   true,
				Writer:            &buf,
				TimeProvider: func() time.Time {
					return time.Time{}
				},
			}
			logger := log.NewLogger(opts)
			writer := logwriter.NewLogWriter(logger)

			var actualMessages []string
			for _, message := range tt.messages {
				b := []byte(message)

				_, err := writer.Write(b)
				assert.NoError(t, err)

				actualMessage := buf.String()
				buf.Reset()
				if actualMessage != "" {
					actualMessages = append(actualMessages, actualMessage)
				}
			}

			err := writer.Close()
			require.NoError(t, err)

			require.Equal(t, tt.expectedMessages, actualMessages)
		})
	}
}

func Test_GivenWriter_WhenStdoutIsUsed_ThenCapturesTheOutput(t *testing.T) {
	tests := []struct {
		name            string
		producer        log.Producer
		loggerType      log.LoggerType
		message         string
		expectedMessage string
	}{
		{
			name:            "ClI console log",
			producer:        log.BitriseCLI,
			loggerType:      log.ConsoleLogger,
			message:         "Test message",
			expectedMessage: "Test message",
		},
		{
			name:            "Step JSON log",
			producer:        log.Step,
			loggerType:      log.JSONLogger,
			message:         "Test message",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"step","level":"normal","message":"Test message"}` + "\n",
		},
		{
			name:            "Empty step JSON log",
			producer:        log.Step,
			loggerType:      log.JSONLogger,
			message:         "",
			expectedMessage: "",
		},
		{
			name:            "New line step JSON log",
			producer:        log.Step,
			loggerType:      log.JSONLogger,
			message:         "\n",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"step","level":"normal","message":"\n"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			opts := log.LoggerOpts{
				LoggerType:        tt.loggerType,
				Producer:          tt.producer,
				ConsoleLoggerOpts: log.ConsoleLoggerOpts{},
				DebugLogEnabled:   true,
				Writer:            &buf,
				TimeProvider: func() time.Time {
					return time.Time{}
				},
			}
			logger := log.NewLogger(opts)
			writer := logwriter.NewLogWriter(logger)

			b := []byte(tt.message)

			_, err := writer.Write(b)
			assert.NoError(t, err)
			require.Equal(t, tt.expectedMessage, buf.String())
		})
	}
}

func Test_GivenWriter_WhenMessageIsWritten_ThenParsesLogLevel(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedMessage string
	}{
		{
			name:            "Normal message without a color literal",
			message:         "This is a normal message without a color literal\n",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"This is a normal message without a color literal\n"}` + "\n",
		},

		{
			name:            "Error message",
			message:         "\u001B[31;1mThis is an error\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"error","message":"This is an error"}` + "\n",
		},
		{
			name:            "Warn message",
			message:         "\u001B[33;1mThis is a warning\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"warn","message":"This is a warning"}` + "\n",
		},
		{
			name:            "Info message",
			message:         "\u001B[34;1mThis is an Info\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"info","message":"This is an Info"}` + "\n",
		},
		{
			name:            "Done message",
			message:         "\u001B[32;1mThis is a done message\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"done","message":"This is a done message"}` + "\n",
		},
		{
			name:            "Debug message",
			message:         "\u001B[35;1mThis is a debug message\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"debug","message":"This is a debug message"}` + "\n",
		},
		{
			name:            "Info message with multiple embedded colors",
			message:         "\u001B[34;1mThis is \u001B[33;1mmulti color \u001B[31;1mInfo message\u001B[0m",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"info","message":"This is \u001b[33;1mmulti color \u001b[31;1mInfo message"}` + "\n",
		},
		{
			name:            "Error message with whitespaces at the end (not a message with log level)",
			message:         "\u001B[31;1mLast error\u001B[0m   \n",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"\u001b[31;1mLast error\u001b[0m   \n"}` + "\n",
		},
		{
			name:            "Error message with whitespaces at the beginning (not a message with log level)",
			message:         "  \u001B[31;1mLast error\u001B[0m   \n",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"  \u001b[31;1mLast error\u001b[0m   \n"}` + "\n",
		},
		{
			name:            "Error message without a closing color literal (not a message with log level)",
			message:         "\u001B[31;1mAnother error\n",
			expectedMessage: `{"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"","level":"normal","message":"\u001b[31;1mAnother error\n"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			opts := log.LoggerOpts{
				LoggerType:        log.JSONLogger,
				ConsoleLoggerOpts: log.ConsoleLoggerOpts{},
				DebugLogEnabled:   true,
				Writer:            &buf,
				TimeProvider: func() time.Time {
					return time.Time{}
				},
			}
			logger := log.NewLogger(opts)
			writer := logwriter.NewLogWriter(logger)

			b := []byte(tt.message)

			_, err := writer.Write(b)
			assert.NoError(t, err)
			err = writer.Close()
			assert.NoError(t, err)
			require.Equal(t, tt.expectedMessage, buf.String())
		})
	}
}

func ExampleNewLogLevelWriter() {
	opts := log.LoggerOpts{
		LoggerType:        log.JSONLogger,
		Producer:          log.BitriseCLI,
		ConsoleLoggerOpts: log.ConsoleLoggerOpts{},
		DebugLogEnabled:   true,
		Writer:            os.Stdout,
		TimeProvider: func() time.Time {
			return time.Time{}
		},
	}
	logger := log.NewLogger(opts)
	writer := logwriter.NewLogWriter(logger)
	cmd := exec.Command("echo", "test")
	cmd.Stdout = writer
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	// Output: {"timestamp":"0001-01-01T00:00:00Z","type":"log","producer":"bitrise_cli","level":"normal","message":"test\n"}
}
