/*
Copyright 2023 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type DataDog struct {
	apiClient *datadog.APIClient
	eventsApi *datadogV1.EventsApi
	apiKey    string
}

// NewDataDog creates a new DataDog provider by mapping the notification provider API to sensible values for the DataDog API.
// url: The DataDog API endpoint to use. Examples: https://api.datadoghq.com, https://api.datadoghq.eu, etc.
// token: The DataDog API key (not the application key).
// headers: A map of extra tags to add to the event
func NewDataDog(address string, proxyUrl string, tlsConfig *tls.Config, token string) (*DataDog, error) {
	conf := datadog.NewConfiguration()

	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	baseUrl, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse address %q: %w", address, err)
	}

	conf.Host = baseUrl.Host
	conf.Scheme = baseUrl.Scheme

	if proxyUrl != "" || tlsConfig != nil {
		transport := &http.Transport{}

		if proxyUrl != "" {
			proxy, err := url.Parse(proxyUrl)
			if err != nil {
				return nil, fmt.Errorf("failed to parse proxy URL %q: %w", proxyUrl, err)
			}

			transport.Proxy = http.ProxyURL(proxy)
		}

		if tlsConfig != nil {
			transport.TLSClientConfig = tlsConfig
		}

		conf.HTTPClient = &http.Client{
			Transport: transport,
		}
	}

	apiClient := datadog.NewAPIClient(conf)
	eventsApi := datadogV1.NewEventsApi(apiClient)

	return &DataDog{
		apiClient: apiClient,
		eventsApi: eventsApi,
		apiKey:    token,
	}, nil
}

func (d *DataDog) Post(ctx context.Context, event eventv1.Event) error {
	dataDogEvent := d.toDataDogEvent(&event)

	_, _, err := d.eventsApi.CreateEvent(d.dataDogCtx(ctx), dataDogEvent)
	if err != nil {
		return fmt.Errorf("failed to post event to DataDog: %w", err)
	}

	return nil
}

// dataDogCtx returns a context with the DataDog API key set.
// This is one way to authenticate with the DataDog API.
func (d *DataDog) dataDogCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {
			Key: d.apiKey,
		},
	})
}

// toDataDogEvent converts an eventv1.Event to a datadogV1.EventCreateRequest.
func (d *DataDog) toDataDogEvent(event *eventv1.Event) datadogV1.EventCreateRequest {
	return datadogV1.EventCreateRequest{
		// Note: Title's printf format matches other events from datadog's kubernetes integration
		Title: fmt.Sprintf("Events from the %s %s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name, event.InvolvedObject.Namespace),
		Text:  event.Message,
		Tags:  d.toDataDogTags(event),
		// fluxcd matches the name datadog picked for their flux integration: https://docs.datadoghq.com/integrations/fluxcd/
		SourceTypeName: strPtr("fluxcd"),
		DateHappened:   int64Ptr(event.Timestamp.Unix()),
		AlertType:      toDataDogAlertType(event),
	}
}

// toDataDogTags parses an eventv1.Event to return a slice of tags.
// We set kind, name, and namespace to the appropriate values of the involved object.
func (d *DataDog) toDataDogTags(event *eventv1.Event) []string {
	// Note: Datadog's built in kubernetes tagging is documented here: https://docs.datadoghq.com/containers/kubernetes/tag/?tab=containerizedagent#out-of-the-box-tags
	tags := []string{
		fmt.Sprintf("flux_reporting_controller:%s", event.ReportingController),
		fmt.Sprintf("flux_reason:%s", event.Reason),
		// Note: DataDog standardizes kubernetes tags as "kube_*": https://github.com/DataDog/datadog-agent/blob/82dc933aa86de037c70fe960384aa06a62e457a8/pkg/collector/corechecks/cluster/kubernetesapiserver/events_common.go#L48
		fmt.Sprintf("kube_kind:%s", event.InvolvedObject.Kind),
		fmt.Sprintf("kube_name:%s", event.InvolvedObject.Name),
		fmt.Sprintf("kube_namespace:%s", event.InvolvedObject.Namespace),
	}

	// add extra tags from event metadata
	for k, v := range event.Metadata {
		tags = append(tags, fmt.Sprintf("%s:%s", k, v))
	}

	// Note: https://docs.datadoghq.com/getting_started/tagging/
	//  "Tags are converted to lowercase"
	//  To keep the events consistent, we run toLower on all input strings.
	for idx := range tags {
		tags[idx] = strings.ToLower(tags[idx])
	}

	return tags
}

// toDataDogAlertType parses an eventv1.Event to return a datadogV1.EventAlertType.
func toDataDogAlertType(event *eventv1.Event) *datadogV1.EventAlertType {
	if event.Severity == eventv1.EventSeverityError {
		return dataDogEventAlertTypePtr(datadogV1.EVENTALERTTYPE_ERROR)
	}

	return dataDogEventAlertTypePtr(datadogV1.EVENTALERTTYPE_INFO)
}

func dataDogEventAlertTypePtr(t datadogV1.EventAlertType) *datadogV1.EventAlertType {
	return &t
}
