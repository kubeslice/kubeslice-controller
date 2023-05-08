/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
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

package metrics

import (
	"fmt"
	"github.com/kubeslice/kubeslice-controller/util"
	"github.com/prometheus/client_golang/prometheus"
)

// IMetricRecorder is used to record metrics from a component
type IMetricRecorder interface {
	// RecordGaugeMetric is used to record a new gauge metric
	RecordGaugeMetric(metric *prometheus.GaugeVec, labels map[string]string, value float64)
	// RecordCounterMetric is used to record a new counter metric
	RecordCounterMetric(metric *prometheus.CounterVec, labels map[string]string)
	// WithSlice returns a new recorder with slice name added
	WithSlice(string) *MetricRecorder
	// WithNamespace returns a new recorder with namespace name added
	WithNamespace(string) *MetricRecorder
	// WithProject returns a new recorder with project name added
	WithProject(string) *MetricRecorder
}

type MetricRecorder struct {
	Options IMetricRecorderOptions
}

// IMetricRecorderOptions provides a container with config parameters for the Prometheus Exporter
type IMetricRecorderOptions struct {
	// Project is the name of the project
	Project string
	// Slice is the name of the slice
	Slice string
	// Namespace is the namespace this metric recorder corresponds to
	Namespace string
}

func (mr *MetricRecorder) RecordGaugeMetric(metric *prometheus.GaugeVec, labels map[string]string, value float64) {
	metric.With(mr.getCurryLabels(labels)).Set(value)
}

func (mr *MetricRecorder) RecordCounterMetric(metric *prometheus.CounterVec, labels map[string]string) {
	fmt.Println(metric)
	fmt.Println(labels)
	metric.With(mr.getCurryLabels(labels)).Inc()
}

func (mr *MetricRecorder) WithSlice(slice string) *MetricRecorder {
	mr.Options.Slice = slice
	return mr
}

func (mr *MetricRecorder) WithNamespace(ns string) *MetricRecorder {
	mr.Options.Namespace = ns
	return mr
}

func (mr *MetricRecorder) WithProject(project string) *MetricRecorder {
	mr.Options.Project = project
	return mr
}

// Adds slice labels to the list of labels provided
// Returns the new set of labels and slice labels
func (mr *MetricRecorder) getCurryLabels(labels map[string]string) prometheus.Labels {
	sliceName := util.NotApplicable
	if mr.Options.Slice != "" {
		sliceName = mr.Options.Slice
	}
	pl := prometheus.Labels{
		"slice_name":                 sliceName,
		"slice_project":              mr.Options.Project,
		"slice_cluster":              util.ClusterController,
		"slice_namespace":            mr.Options.Namespace,
		"slice_reporting_controller": util.InstanceController,
	}

	for k, v := range labels {
		pl[k] = v
	}

	for k, v := range pl {
		// Remove labels if value is empty
		if v == "" {
			delete(pl, k)
			continue
		}
	}
	return pl
}
