/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package events

import (
	"fmt"
	"github.com/openziti/metrics/metrics_pb"
	"github.com/openziti/ziti/controller/event"
	"github.com/pkg/errors"
	"reflect"
	"time"
)

func (self *Dispatcher) AddUsageEventHandler(handler event.UsageEventHandler) {
	self.usageEventHandlers.Append(handler)
}

func (self *Dispatcher) RemoveUsageEventHandler(handler event.UsageEventHandler) {
	self.usageEventHandlers.DeleteIf(func(val event.UsageEventHandler) bool {
		if val == handler {
			return true
		}
		if w, ok := val.(event.UsageEventHandlerWrapper); ok {
			return w.IsWrapping(handler)
		}
		return false
	})
}

func (self *Dispatcher) AddUsageEventV3Handler(handler event.UsageEventV3Handler) {
	self.usageEventV3Handlers.Append(handler)
}

func (self *Dispatcher) RemoveUsageEventV3Handler(handler event.UsageEventV3Handler) {
	self.usageEventV3Handlers.DeleteIf(func(val event.UsageEventV3Handler) bool {
		if val == handler {
			return true
		}
		if w, ok := val.(event.UsageEventV3HandlerWrapper); ok {
			return w.IsWrapping(handler)
		}
		return false
	})
}

func (self *Dispatcher) AcceptUsageEvent(event *event.UsageEventV2) {
	go func() {
		for _, handler := range self.usageEventHandlers.Value() {
			handler.AcceptUsageEvent(event)
		}
	}()
}

func (self *Dispatcher) AcceptUsageEventV3(event *event.UsageEventV3) {
	go func() {
		for _, handler := range self.usageEventV3Handlers.Value() {
			handler.AcceptUsageEventV3(event)
		}
	}()
}

func (self *Dispatcher) registerUsageEventHandler(eventType string, val interface{}, config map[string]interface{}) error {
	version := 2

	if configVal, found := config["version"]; found {
		strVal := fmt.Sprintf("%v", configVal)
		if strVal == "2" {
			version = 2
		} else if strVal == "3" {
			version = 3
		} else {
			return errors.Errorf("unsupported usage version: %v", version)
		}
	}

	if version == 2 {
		handler, ok := val.(event.UsageEventHandler)
		if !ok {
			return errors.Errorf("type %v doesn't implement github.com/openziti/ziti/controller/event/UsageEventHandler interface.", reflect.TypeOf(val))
		}
		if eventType != event.UsageEventNS {
			handler = &usageEventV2OldNsAdapter{
				namespace: eventType,
				wrapped:   handler,
			}
		}
		self.AddUsageEventHandler(handler)
	} else {
		handler, ok := val.(event.UsageEventV3Handler)
		if !ok {
			return errors.Errorf("type %v doesn't implement github.com/openziti/ziti/controller/event/UsageEventV3Handler interface.", reflect.TypeOf(val))
		}

		if eventType != event.UsageEventNS {
			handler = &usageEventV3OldNsAdapter{
				namespace: eventType,
				wrapped:   handler,
			}
		}

		if includeListVal, found := config["include"]; found {
			includes := map[string]struct{}{}
			if list, ok := includeListVal.([]interface{}); ok {
				for _, includeVal := range list {
					if include, ok := includeVal.(string); ok {
						includes[include] = struct{}{}
					} else {
						return errors.Errorf("invalid value type [%T] for usage include list, must be string list", val)
					}
				}
			} else {
				return errors.Errorf("invalid value type [%T] for usage include list, must be string list", val)
			}

			if len(includes) == 0 {
				return errors.Errorf("no values provided in include list for usage events, either drop includes stanza or provide at least one usage type to include")
			}

			handler = &filteredUsageV3EventHandler{
				include: includes,
				wrapped: handler,
			}
		}

		self.AddUsageEventV3Handler(handler)
	}

	return nil
}

func (self *Dispatcher) unregisterUsageEventHandler(val interface{}) {
	if handler, ok := val.(event.UsageEventHandler); ok {
		self.RemoveUsageEventHandler(handler)
	}

	if handler, ok := val.(event.UsageEventV3Handler); ok {
		self.RemoveUsageEventV3Handler(handler)
	}
}

func (self *Dispatcher) initUsageEvents() {
	self.AddMetricsMessageHandler(&usageEventAdapter{
		dispatcher: self,
	})
}

type usageEventAdapter struct {
	dispatcher *Dispatcher
}

func (self *usageEventAdapter) AcceptMetricsMsg(message *metrics_pb.MetricsMessage) {
	if message.DoNotPropagate {
		return
	}

	if len(self.dispatcher.usageEventHandlers.Value()) > 0 {
		for name, interval := range message.IntervalCounters {
			for _, bucket := range interval.Buckets {
				for circuitId, usage := range bucket.Values {
					evt := &event.UsageEventV2{
						Namespace:        event.UsageEventNS,
						EventSrcId:       self.dispatcher.ctrlId,
						Timestamp:        time.Now(),
						Version:          event.UsageEventsVersion,
						EventType:        name,
						SourceId:         message.SourceId,
						CircuitId:        circuitId,
						Usage:            usage,
						IntervalStartUTC: bucket.IntervalStartUTC,
						IntervalLength:   interval.IntervalLength,
					}
					self.dispatcher.AcceptUsageEvent(evt)
				}
			}
		}
		for _, interval := range message.UsageCounters {
			for circuitId, bucket := range interval.Buckets {
				for usageType, usage := range bucket.Values {
					evt := &event.UsageEventV2{
						Namespace:        event.UsageEventNS,
						EventSrcId:       self.dispatcher.ctrlId,
						Timestamp:        time.Now(),
						Version:          event.UsageEventsVersion,
						EventType:        "usage." + usageType,
						SourceId:         message.SourceId,
						CircuitId:        circuitId,
						Usage:            usage,
						IntervalStartUTC: interval.IntervalStartUTC,
						IntervalLength:   interval.IntervalLength,
						Tags:             bucket.Tags,
					}
					self.dispatcher.AcceptUsageEvent(evt)
				}
			}
		}
	}

	if len(self.dispatcher.usageEventV3Handlers.Value()) > 0 {
		for name, interval := range message.IntervalCounters {
			for _, bucket := range interval.Buckets {
				for circuitId, usage := range bucket.Values {
					evt := &event.UsageEventV3{
						Namespace:  event.UsageEventNS,
						EventSrcId: self.dispatcher.ctrlId,
						Timestamp:  time.Now(),
						Version:    3,
						SourceId:   message.SourceId,
						CircuitId:  circuitId,
						Usage: map[string]uint64{
							name: usage,
						},
						IntervalStartUTC: bucket.IntervalStartUTC,
						IntervalLength:   interval.IntervalLength,
					}
					self.dispatcher.AcceptUsageEventV3(evt)
				}
			}
		}

		for _, interval := range message.UsageCounters {
			for circuitId, bucket := range interval.Buckets {
				evt := &event.UsageEventV3{
					Namespace:        event.UsageEventNS,
					EventSrcId:       self.dispatcher.ctrlId,
					Timestamp:        time.Now(),
					Version:          3,
					SourceId:         message.SourceId,
					CircuitId:        circuitId,
					Usage:            bucket.Values,
					IntervalStartUTC: interval.IntervalStartUTC,
					IntervalLength:   interval.IntervalLength,
					Tags:             bucket.Tags,
				}
				self.dispatcher.AcceptUsageEventV3(evt)
			}
		}
	}
}

type filteredUsageV3EventHandler struct {
	include map[string]struct{}
	wrapped event.UsageEventV3Handler
}

func (self *filteredUsageV3EventHandler) IsWrapping(value event.UsageEventV3Handler) bool {
	if self.wrapped == value {
		return true
	}
	if w, ok := self.wrapped.(event.UsageEventV3HandlerWrapper); ok {
		return w.IsWrapping(value)
	}
	return false
}

func (self *filteredUsageV3EventHandler) AcceptUsageEventV3(event *event.UsageEventV3) {
	usage := map[string]uint64{}
	for k, v := range event.Usage {
		if _, found := self.include[k]; found {
			usage[k] = v
		}
	}
	// nothing passed filter, skip event
	if len(usage) == 0 {
		return
	}

	// nothing got filtered out, pass through unchanged
	if len(usage) == len(event.Usage) {
		self.wrapped.AcceptUsageEventV3(event)
		return
	}

	newEvent := *event
	newEvent.Usage = usage
	self.wrapped.AcceptUsageEventV3(&newEvent)
}

type usageEventV2OldNsAdapter struct {
	namespace string
	wrapped   event.UsageEventHandler
}

func (self *usageEventV2OldNsAdapter) AcceptUsageEvent(event *event.UsageEventV2) {
	nsEvent := *event
	nsEvent.Namespace = self.namespace
	self.wrapped.AcceptUsageEvent(&nsEvent)
}

func (self *usageEventV2OldNsAdapter) IsWrapping(value event.UsageEventHandler) bool {
	if self.wrapped == value {
		return true
	}
	if w, ok := self.wrapped.(event.UsageEventHandlerWrapper); ok {
		return w.IsWrapping(value)
	}
	return false
}

type usageEventV3OldNsAdapter struct {
	namespace string
	wrapped   event.UsageEventV3Handler
}

func (self *usageEventV3OldNsAdapter) AcceptUsageEventV3(evt *event.UsageEventV3) {
	nsEvent := *evt
	nsEvent.Namespace = self.namespace
	self.wrapped.AcceptUsageEventV3(&nsEvent)
}

func (self *usageEventV3OldNsAdapter) IsWrapping(value event.UsageEventV3Handler) bool {
	if self.wrapped == value {
		return true
	}
	if w, ok := self.wrapped.(event.UsageEventV3HandlerWrapper); ok {
		return w.IsWrapping(value)
	}
	return false
}
