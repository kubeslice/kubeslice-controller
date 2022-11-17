/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package util

import (
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var logLevelSeverity = map[string]zapcore.Level{
	"debug":   zapcore.DebugLevel,
	"info":    zapcore.InfoLevel,
	"warning": zapcore.WarnLevel,
	"error":   zapcore.ErrorLevel,
}

// NewLogger Creates a new SugaredLogger instance with predefined standard fields
// SugaredLogger makes it easy to use structured logging with logging levels and additional fields
func NewLogger() *uzap.SugaredLogger {

	// info and debug level enabler
	debugInfoLevel := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= logLevelSeverity[LoglevelString] && level < zapcore.ErrorLevel
	})

	// error and fatal level enabler
	errorFatalLevel := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapcore.ErrorLevel
	})

	// write syncers
	stdoutSyncer := zapcore.Lock(os.Stdout)
	stderrSyncer := zapcore.Lock(os.Stderr)

	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = zapcore.RFC3339TimeEncoder
	configLog.LevelKey = "severity"
	configLog.MessageKey = "message"
	configLog.TimeKey = "time"

	// tee core
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(configLog),
			stdoutSyncer,
			debugInfoLevel,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(configLog),
			stderrSyncer,
			errorFatalLevel,
		),
	)

	// finally return the logger with the tee core
	return uzap.New(core).Sugar()
}
