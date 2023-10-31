/*
Copyright 2020 The Flux authors

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
	"crypto/x509"
	"fmt"

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta2"
)

type Factory struct {
	URL         string
	ProxyURL    string
	Username    string
	Channel     string
	Token       string
	Headers     map[string]string
	CertPool    *x509.CertPool
	Password    string
	ProviderUID string
}

func NewFactory(url string,
	proxy string,
	username string,
	channel string,
	token string,
	headers map[string]string,
	certPool *x509.CertPool,
	password string,
	providerUID string) *Factory {
	return &Factory{
		URL:         url,
		ProxyURL:    proxy,
		Channel:     channel,
		Username:    username,
		Token:       token,
		Headers:     headers,
		CertPool:    certPool,
		Password:    password,
		ProviderUID: providerUID,
	}
}

func (f Factory) Notifier(provider string) (Interface, error) {
	if f.URL == "" {
		return &NopNotifier{}, nil
	}

	var n Interface
	var err error
	switch provider {
	case apiv1.GenericProvider:
		n, err = NewForwarder(f.URL, f.ProxyURL, f.Headers, f.CertPool, nil)
	case apiv1.GenericHMACProvider:
		n, err = NewForwarder(f.URL, f.ProxyURL, f.Headers, f.CertPool, []byte(f.Token))
	case apiv1.SlackProvider:
		n, err = NewSlack(f.URL, f.ProxyURL, f.Token, f.CertPool, f.Username, f.Channel)
	case apiv1.DiscordProvider:
		n, err = NewDiscord(f.URL, f.ProxyURL, f.Username, f.Channel)
	case apiv1.RocketProvider:
		n, err = NewRocket(f.URL, f.ProxyURL, f.CertPool, f.Username, f.Channel)
	case apiv1.MSTeamsProvider:
		n, err = NewMSTeams(f.URL, f.ProxyURL, f.CertPool)
	case apiv1.GitHubProvider:
		n, err = NewGitHub(f.ProviderUID, f.URL, f.Token, f.CertPool)
	case apiv1.GitHubDispatchProvider:
		n, err = NewGitHubDispatch(f.URL, f.Token, f.CertPool)
	case apiv1.GitLabProvider:
		n, err = NewGitLab(f.ProviderUID, f.URL, f.Token, f.CertPool)
	case apiv1.GiteaProvider:
		n, err = NewGitea(f.ProviderUID, f.URL, f.Token, f.CertPool)
	case apiv1.BitbucketServerProvider:
		n, err = NewBitbucketServer(f.ProviderUID, f.URL, f.Token, f.CertPool, f.Username, f.Password)
	case apiv1.BitbucketProvider:
		n, err = NewBitbucket(f.ProviderUID, f.URL, f.Token, f.CertPool)
	case apiv1.AzureDevOpsProvider:
		n, err = NewAzureDevOps(f.ProviderUID, f.URL, f.Token, f.CertPool)
	case apiv1.GoogleChatProvider:
		n, err = NewGoogleChat(f.URL, f.ProxyURL)
	case apiv1.GooglePubSubProvider:
		n, err = NewGooglePubSub(f.URL, f.Channel, f.Token, f.Headers)
	case apiv1.WebexProvider:
		n, err = NewWebex(f.URL, f.ProxyURL, f.CertPool, f.Channel, f.Token)
	case apiv1.SentryProvider:
		n, err = NewSentry(f.CertPool, f.URL, f.Channel)
	case apiv1.AzureEventHubProvider:
		n, err = NewAzureEventHub(f.URL, f.Token, f.Channel)
	case apiv1.TelegramProvider:
		n, err = NewTelegram(f.Channel, f.Token)
	case apiv1.LarkProvider:
		n, err = NewLark(f.URL)
	case apiv1.Matrix:
		n, err = NewMatrix(f.URL, f.Token, f.Channel, f.CertPool)
	case apiv1.OpsgenieProvider:
		n, err = NewOpsgenie(f.URL, f.ProxyURL, f.CertPool, f.Token)
	case apiv1.AlertManagerProvider:
		n, err = NewAlertmanager(f.URL, f.ProxyURL, f.CertPool)
	case apiv1.GrafanaProvider:
		n, err = NewGrafana(f.URL, f.ProxyURL, f.Token, f.CertPool, f.Username, f.Password)
	case apiv1.PagerDutyProvider:
		n, err = NewPagerDuty(f.URL, f.ProxyURL, f.CertPool, f.Channel)
	case apiv1.DataDogProvider:
		n, err = NewDataDog(f.URL, f.ProxyURL, f.CertPool, f.Token)
	default:
		err = fmt.Errorf("provider %s not supported", provider)
	}

	if err != nil {
		n = &NopNotifier{}
	}
	return n, err
}
