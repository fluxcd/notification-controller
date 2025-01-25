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

	apiv1 "github.com/fluxcd/notification-controller/api/v1beta3"
)

var (
	// notifiers is a map of notifier names to factory functions.
	notifiers = notifierMap{
		// GenericProvider is the default notifier
		apiv1.GenericProvider:         genericNotifierFunc,
		apiv1.GenericHMACProvider:     genericHMACNotifierFunc,
		apiv1.SlackProvider:           slackNotifierFunc,
		apiv1.DiscordProvider:         discordNotifierFunc,
		apiv1.RocketProvider:          rocketNotifierFunc,
		apiv1.MSTeamsProvider:         msteamsNotifierFunc,
		apiv1.GoogleChatProvider:      googleChatNotifierFunc,
		apiv1.GooglePubSubProvider:    googlePubSubNotifierFunc,
		apiv1.WebexProvider:           webexNotifierFunc,
		apiv1.SentryProvider:          sentryNotifierFunc,
		apiv1.AzureEventHubProvider:   azureEventHubNotifierFunc,
		apiv1.TelegramProvider:        telegramNotifierFunc,
		apiv1.LarkProvider:            larkNotifierFunc,
		apiv1.Matrix:                  matrixNotifierFunc,
		apiv1.OpsgenieProvider:        opsgenieNotifierFunc,
		apiv1.AlertManagerProvider:    alertmanagerNotifierFunc,
		apiv1.GrafanaProvider:         grafanaNotifierFunc,
		apiv1.PagerDutyProvider:       pagerDutyNotifierFunc,
		apiv1.DataDogProvider:         dataDogNotifierFunc,
		apiv1.NATSProvider:            natsNotifierFunc,
		apiv1.GitHubProvider:          gitHubNotifierFunc,
		apiv1.GitHubDispatchProvider:  gitHubDispatchNotifierFunc,
		apiv1.GitLabProvider:          gitLabNotifierFunc,
		apiv1.GiteaProvider:           giteaNotifierFunc,
		apiv1.BitbucketServerProvider: bitbucketServerNotifierFunc,
		apiv1.BitbucketProvider:       bitbucketNotifierFunc,
		apiv1.AzureDevOpsProvider:     azureDevOpsNotifierFunc,
	}
)

// notifierMap is a map of provider names to notifier factory functions
type notifierMap map[string]factoryFunc

// factoryFunc is a factory function that creates a new notifier
type factoryFunc func(opts notifierOptions) (Interface, error)

type notifierOptions struct {
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

type Factory struct {
	notifierOptions
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
		notifierOptions: notifierOptions{
			URL:         url,
			ProxyURL:    proxy,
			Username:    username,
			Channel:     channel,
			Token:       token,
			Headers:     headers,
			CertPool:    certPool,
			Password:    password,
			ProviderUID: providerUID,
		},
	}
}

func (f Factory) Notifier(provider string) (Interface, error) {
	if f.URL == "" {
		return &NopNotifier{}, nil
	}

	var (
		n   Interface
		err error
	)
	if notifier, ok := notifiers[provider]; ok {
		n, err = notifier(f.notifierOptions)
	} else {
		err = fmt.Errorf("provider %s not supported", provider)
	}

	if err != nil {
		n = &NopNotifier{}
	}
	return n, err
}

func genericNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewForwarder(opts.URL, opts.ProxyURL, opts.Headers, opts.CertPool, nil)
}

func genericHMACNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewForwarder(opts.URL, opts.ProxyURL, opts.Headers, opts.CertPool, []byte(opts.Token))
}

func slackNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewSlack(opts.URL, opts.ProxyURL, opts.Token, opts.CertPool, opts.Username, opts.Channel)
}

func discordNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewDiscord(opts.URL, opts.ProxyURL, opts.Username, opts.Channel)
}

func rocketNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewRocket(opts.URL, opts.ProxyURL, opts.CertPool, opts.Username, opts.Channel)
}

func msteamsNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewMSTeams(opts.URL, opts.ProxyURL, opts.CertPool)
}

func googleChatNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGoogleChat(opts.URL, opts.ProxyURL)
}

func googlePubSubNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGooglePubSub(opts.URL, opts.Channel, opts.Token, opts.Headers)
}

func webexNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewWebex(opts.URL, opts.ProxyURL, opts.CertPool, opts.Channel, opts.Token)
}

func sentryNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewSentry(opts.CertPool, opts.URL, opts.Channel)
}

func azureEventHubNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAzureEventHub(opts.URL, opts.Token, opts.Channel)
}

func telegramNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewTelegram(opts.Channel, opts.Token)
}

func larkNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewLark(opts.URL)
}

func matrixNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewMatrix(opts.URL, opts.Token, opts.Channel, opts.CertPool)
}

func opsgenieNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewOpsgenie(opts.URL, opts.ProxyURL, opts.CertPool, opts.Token)
}

func alertmanagerNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAlertmanager(opts.URL, opts.ProxyURL, opts.CertPool, opts.Token)
}

func grafanaNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGrafana(opts.URL, opts.ProxyURL, opts.Token, opts.CertPool, opts.Username, opts.Password)
}

func pagerDutyNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewPagerDuty(opts.URL, opts.ProxyURL, opts.CertPool, opts.Channel)
}

func dataDogNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewDataDog(opts.URL, opts.ProxyURL, opts.CertPool, opts.Token)
}

func natsNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewNATS(opts.URL, opts.Channel, opts.Username, opts.Password)
}

func gitHubNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitHub(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool)
}

func gitHubDispatchNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitHubDispatch(opts.URL, opts.Token, opts.CertPool)
}

func gitLabNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitLab(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool)
}

func giteaNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitea(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool)
}

func bitbucketServerNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewBitbucketServer(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool, opts.Username, opts.Password)
}

func bitbucketNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewBitbucket(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool)
}

func azureDevOpsNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAzureDevOps(opts.ProviderUID, opts.URL, opts.Token, opts.CertPool)
}
