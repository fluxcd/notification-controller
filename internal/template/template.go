package template

import (
	"bytes"

	"github.com/Masterminds/sprig"
	"html/template"

	"github.com/fluxcd/pkg/runtime/events"
)

type templateData struct {
	InvolvedObject templateInvolvedObject
	Message        string
	Reason         string
	Metadata       templateMetadata
}

type templateMetadata struct {
	Revision string
}

type templateInvolvedObject struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

func templateString(event events.Event, tmplString string) (string, error) {
	revString, ok := event.Metadata["revision"]
	if !ok {
		revString = ""
	}

	data := templateData{
		InvolvedObject: templateInvolvedObject{
			APIVersion: event.InvolvedObject.APIVersion,
			Kind:       event.InvolvedObject.Kind,
			Namespace:  event.InvolvedObject.Namespace,
			Name:       event.InvolvedObject.Name,
		},
		Message: event.Message,
		Reason:  event.Reason,
		Metadata: templateMetadata{
			Revision: revString,
		},
	}
	tmpl, err := template.New("base").Funcs(sprig.FuncMap()).Parse(tmplString)
	if err != nil {
		return "", err
	}
	var result bytes.Buffer
	err = tmpl.Execute(&result, data)
	return result.String(), nil
}
