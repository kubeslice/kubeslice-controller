package main

import (
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"gopkg.in/yaml.v2"
)

func main() {
	fmt.Println("generating event schema code from schema file")

	file1 := "controller.yaml"
	pwd, err := os.Getwd()
	handleError(err)
	controllerFilePath := path.Join(pwd, "config/events", file1)
	events, err := parseEvent(controllerFilePath)
	handleError(err)

	t, err := template.ParseFiles("hack/events/templates/schema.tmpl")
	handleError(err)

	f, err := os.Create("events/events_generated.go")
	handleError(err)
	t.Execute(f, events)

	cmt, err := template.ParseFiles("hack/events/templates/config-map.tmpl")
	handleError(err)

	fcm, err := os.Create("config/events/events_config_map.yaml")
	handleError(err)

	cmt.Execute(fcm, events)
}

func handleError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func parseEvent(filepath string) ([]events.EventSchema, error) {
	var eventSchema struct {
		Events []events.EventSchema
	}
	event, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(event, &eventSchema)
	if err != nil {
		return nil, err
	}
	return eventSchema.Events, nil
}
