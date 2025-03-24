package events

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"
)

type EventConfig struct {
	DisabledEvents []string
}

type EventType string

var (
	EventTypeWarning EventType = "Warning"
	EventTypeNormal  EventType = "Normal"
)

type EventName string

type EventSchema struct {
	Name                EventName
	Reason              string
	Action              string
	Type                EventType
	ReportingController string `yaml:"reportingController"`
	Message             string
}

func GetEvent(name EventName, eventsMap map[EventName]*EventSchema) (*EventSchema, error) {
	if _, ok := eventsMap[name]; !ok {
		return nil, fmt.Errorf("invalid event")
	}
	return eventsMap[name], nil
}

func IsEventDisabled(name EventName) bool {
	return isDisabledInFile(name, "/events/event-schema/controller.yaml") ||
		isDisabledInFile(name, "/events/event-schema/worker.yaml")
}

func isDisabledInFile(name EventName, filepath string) bool {
	configs, err := parseConfig(filepath)
	if err != nil {
		return false
	}
	for _, config := range configs {
		if config == string(name) {
			return true
		}
	}
	return false
}

func parseConfig(filepath string) ([]string, error) {
	var eventConfig EventConfig
	event, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(event, &eventConfig)
	if err != nil {
		return nil, err
	}
	return eventConfig.DisabledEvents, nil
}
