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

package events

import (
	b64 "encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/golang/groupcache/lru"

	"github.com/kubeslice/kubeslice-monitoring/pkg/logger"

	"go.uber.org/zap"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EventRecorder is used to record events from a component
type EventRecorder interface {
	// RecordEvent is used to record a new event
	RecordEvent(context.Context, *Event) error
	// WithSlice returns a new recorder with slice name added
	WithSlice(string) EventRecorder
	// WithNamespace returns a new recorder with namespace name added
	WithNamespace(string) EventRecorder
	// WithProject returns a new recorder with project name added
	WithProject(string) EventRecorder
}

func NewEventRecorder(c client.Writer, s *runtime.Scheme, em map[EventName]*EventSchema, o EventRecorderOptions) EventRecorder {
	log := logger.NewLogger().With("name", o.Component)
	return &eventRecorder{
		Client:    c,
		Scheme:    s,
		EventsMap: em,
		Options:   o,
		Logger:    log,
		cache:     lru.New(4096),
	}
}

var _ EventRecorder = (*eventRecorder)(nil)

type EventRecorderOptions struct {
	// Version is the version of the component
	Version string
	// Cluster  is the name of the cluster
	Cluster string
	// Project is the name of the project
	Project string
	// Slice is the name of the slice (optional)
	Slice string
	// Namespace is the namespace this event recorder corresponds to (optional)
	Namespace string
	// Component is the component which uses the event recorder
	Component string
}

type Event struct {
	// Object is the reference of the k8s object which this event corresponds to
	Object runtime.Object
	// RelatedObject is the ref to related object (optional)
	RelatedObject runtime.Object
	// ReportingInstance is the name of the reporting instance
	ReportingInstance string
	// Name is the name of the event to be looked up in event schema
	Name EventName
}

// getEventKey builds unique event key based on source, involvedObject, reason, message
func GetEventKey(event *corev1.Event) string {
	return b64.StdEncoding.EncodeToString(
		[]byte(
			strings.Join([]string{
				event.Source.Component,
				event.Source.Host,
				event.InvolvedObject.Kind,
				event.InvolvedObject.Namespace,
				event.InvolvedObject.Name,
				event.InvolvedObject.FieldPath,
				string(event.InvolvedObject.UID),
				event.InvolvedObject.APIVersion,
				event.Type,
				event.Reason,
				event.Message,
				event.Labels["sliceName"],
				event.Labels["sliceCluster"],
				event.Labels["sliceProject"],
				event.Labels["eventTitle"],
			},
				""),
		),
	)
}

type eventRecorder struct {
	Client    client.Writer
	Logger    *zap.SugaredLogger
	Scheme    *runtime.Scheme
	EventsMap map[EventName]*EventSchema
	Options   EventRecorderOptions
	cache     *lru.Cache   // cache of last seen events
	cacheLock sync.RWMutex // mutex to synchronize access to cache
}

func (er *eventRecorder) Copy() *eventRecorder {
	return &eventRecorder{
		Client:    er.Client,
		Logger:    er.Logger,
		Scheme:    er.Scheme,
		EventsMap: er.EventsMap,
		cache:     er.cache,
		Options: EventRecorderOptions{
			Version:   er.Options.Version,
			Cluster:   er.Options.Cluster,
			Project:   er.Options.Project,
			Slice:     er.Options.Slice,
			Namespace: er.Options.Namespace,
			Component: er.Options.Component,
		},
	}
}

// WithSlice returns a new recorder with added slice name for raising events
func (er *eventRecorder) WithSlice(slice string) EventRecorder {
	e := er.Copy()
	e.Options.Slice = slice
	return e
}

// WithNamespace returns a new recorder with added namespace name
// If namespace is not provided, recorder will use the object namespace
func (er *eventRecorder) WithNamespace(ns string) EventRecorder {
	e := er.Copy()
	e.Options.Namespace = ns
	return e
}

// WithProject returns a new recorder with added project name
func (er *eventRecorder) WithProject(project string) EventRecorder {
	e := er.Copy()
	e.Options.Project = project
	return e
}

// RecordEvent raises a new event with the given fields
func (er *eventRecorder) RecordEvent(ctx context.Context, e *Event) error {
	ref, err := reference.GetReference(er.Scheme, e.Object)
	if err != nil {
		er.Logger.With("error", err).Error("Unable to parse event obj reference")
		return err
	}

	if ref.Namespace == "" {
		er.Logger.Debugf("reference namespace is empty using name instead ", ref.Name)
		ref.Namespace = ref.Name
	}

	ns := er.Options.Namespace
	if ns == "" {
		ns = ref.Namespace
	}

	if IsEventDisabled(e.Name) {
		er.Logger.Infof("Event disabled for %s", e.Name)
		return nil
	}

	event, err := GetEvent(e.Name, er.EventsMap)
	if err != nil {
		er.Logger.With("error", err).Error("Unable to get event")
		return err
	}
	t := metav1.Now()

	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: ns,
			Labels: map[string]string{
				"sliceCluster":            er.Options.Cluster,
				"sliceNamespace":          ns,
				"sliceName":               er.Options.Slice,
				"sliceProject":            er.Options.Project,
				"reportingControllerName": event.ReportingController,
				"eventTitle":              string(event.Name),
			},
			Annotations: map[string]string{
				"release": er.Options.Version,
			},
		},
		InvolvedObject:      *ref,
		EventTime:           metav1.NowMicro(),
		Reason:              event.Reason,
		Message:             event.Message,
		LastTimestamp:       t,
		Count:               1,
		ReportingController: event.ReportingController,
		ReportingInstance:   e.ReportingInstance,
		Source: corev1.EventSource{
			Component: er.Options.Component,
		},
		Action: event.Action,
		Type:   string(event.Type),
	}

	if e.RelatedObject != nil {
		related, err := reference.GetReference(er.Scheme, e.RelatedObject)
		if err != nil {
			er.Logger.With("error", err).Error("Unable to parse event related obj reference")
			return err
		}
		ev.Related = related
	}

	// Check if there is already an event of the same type in the cache
	if er.cache == nil {
		er.cache = lru.New(4096)
	}
	key := GetEventKey(ev)
	er.Logger.Debugf("event key %s", key)
	er.cacheLock.Lock()
	defer er.cacheLock.Unlock()
	lastSeenEvent, ok := er.cache.Get(key)
	if !ok {
		ev.FirstTimestamp = t
		if err := er.Client.Create(ctx, ev); err != nil {
			er.Logger.With("error", err, "event", ev).Error("Unable to create event")
			return err
		} else {
			er.cache.Add(key, ev)
		}
		er.Logger.Infof("event has been created object: %s/%s kind: %s type: %s reason %s message %s",
			ev.InvolvedObject.Namespace, ev.InvolvedObject.Name, ev.InvolvedObject.Kind, ev.Type, ev.Reason, ev.Message)
	} else {
		// event already present in cache
		e := lastSeenEvent.(*corev1.Event)
		e.Count++
		e.LastTimestamp = t
		if err := er.Client.Update(ctx, e); err != nil {
			er.Logger.With("error", err, "event", ev).Error("Unable to update event")
			return err
		}
		// update the cache
		er.cache.Add(key, e)
		er.Logger.Debugf("event has been created object: %s/%s kind: %s type: %s reason %s message %s",
			ev.InvolvedObject.Namespace, ev.InvolvedObject.Name, ev.InvolvedObject.Kind, ev.Type, ev.Reason, ev.Message)
	}
	return nil
}
