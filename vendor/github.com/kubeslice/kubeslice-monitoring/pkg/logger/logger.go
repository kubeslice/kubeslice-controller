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

package logger

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logLevel string
	// ControlPlaneNamespace is the namespace where slice operator is running
	ControlPlaneNamespace = "kubeslice-system"
	// Name of the cluster
	ClusterName = os.Getenv("CLUSTER_NAME")
)

var logLevelSeverity = map[string]zapcore.Level{
	"DEBUG":   zapcore.DebugLevel,
	"INFO":    zapcore.InfoLevel,
	"WARNING": zapcore.WarnLevel,
	"ERROR":   zapcore.ErrorLevel,
}

type loggerKey struct{}

func init() {
	logLevel = os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
}

// WithLogger takes in a context and returns a context with key as loggerKey{} and value as loggger(of type logr.Logger) passed
func WithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns a logger from the context.
func FromContext(ctx context.Context) logr.Logger {

	if v, ok := ctx.Value(loggerKey{}).(logr.Logger); ok {
		return v
	}

	return NewWrappedLogger()
}

// Creates a new zap logger, wraps it to logr.Logger using zapr
// Required for controller-runtime logging
func NewWrappedLogger() logr.Logger {
	logger := NewLogger()

	return zapr.NewLogger(logger.Desugar())
}

// NewLogger Creates a new SugaredLogger instance with predefined standard fields
// SugaredLogger makes it easy to use structured logging with logging levels and additional fields
func NewLogger() *uzap.SugaredLogger {

	// info and debug level enabler
	debugInfoLevel := uzap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= logLevelSeverity[logLevel] && level < zapcore.ErrorLevel
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

	// finally construct the logger with the tee core
	logger := uzap.New(core).Sugar()

	return logger.With("namespace", ControlPlaneNamespace, "sliceCluster", ClusterName)
}
