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

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

type Factory struct {
	URL      string
	ProxyURL string
	Username string
	Channel  string
	Token    string
	CertPool *x509.CertPool
}

func NewFactory(url string, proxy string, username string, channel string, token string, certPool *x509.CertPool) *Factory {
	return &Factory{
		URL:      url,
		ProxyURL: proxy,
		Channel:  channel,
		Username: username,
		Token:    token,
		CertPool: certPool,
	}
}

func (f Factory) Notifier(provider string) (Interface, error) {
	if f.URL == "" {
		return &NopNotifier{}, nil
	}

	var n Interface
	var err error
	switch provider {
	case v1beta1.GenericProvider:
		n, err = NewForwarder(f.URL, f.ProxyURL, f.CertPool)
	case v1beta1.SlackProvider:
		n, err = NewSlack(f.URL, f.ProxyURL, f.CertPool, f.Username, f.Channel)
	case v1beta1.DiscordProvider:
		n, err = NewDiscord(f.URL, f.ProxyURL, f.Username, f.Channel)
	case v1beta1.RocketProvider:
		n, err = NewRocket(f.URL, f.ProxyURL, f.CertPool, f.Username, f.Channel)
	case v1beta1.MSTeamsProvider:
		n, err = NewMSTeams(f.URL, f.ProxyURL)
	case v1beta1.GitHubProvider:
		n, err = NewGitHub(f.URL, f.Token, f.CertPool)
	case v1beta1.GitLabProvider:
		n, err = NewGitLab(f.URL, f.Token, f.CertPool)
	case v1beta1.BitbucketProvider:
		n, err = NewBitbucket(f.URL, f.Token, f.CertPool)
	case v1beta1.AzureDevOpsProvider:
		n, err = NewAzureDevOps(f.URL, f.Token, f.CertPool)
	case v1beta1.GoogleChatProvider:
		n, err = NewGoogleChat(f.URL, f.ProxyURL)
	case v1beta1.WebexProvider:
		n, err = NewWebex(f.URL, f.ProxyURL, f.CertPool)
	case v1beta1.SentryProvider:
		n, err = NewSentry(f.CertPool, f.URL, f.Channel)
	case v1beta1.AzureEventHubProvider:
		n, err = NewAzureEventHub(f.URL, f.Token, f.Channel)
	case v1beta1.TelegramProvider:
		n, err = NewTelegram(f.Channel, f.Token)
	case v1beta1.LarkProvider:
		n, err = NewLark(f.URL)
	case v1beta1.Matrix:
		n, err = NewMatrix(f.URL, f.Token, f.Channel)
	case v1beta1.OpsgenieProvider:
		n, err = NewOpsgenie(f.URL, f.ProxyURL, f.CertPool, f.Token)
	default:
		err = fmt.Errorf("provider %s not supported", provider)
	}

	if err != nil {
		n = &NopNotifier{}
	}
	return n, err
}
