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
	"fmt"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

// Factory represents a factory for creating notification clients for sending
// events to various services.
type Factory struct {
	URL           string
	ProxyURL      string
	Username      string
	Channel       string
	Token         string
	SigningSecret string
}

// NewFactory creates and returns a new Factory.
//
// url is a string witht the url to send the notification to, if empty, a
// NopProvider is returned by the factory.
// proxy is an optional string, that configures whether or not the client is
// uses a proxy to send the notification.
// username and channel are used by the Slack, Discord and Rocket providers.
// token is used by the GitHub, GitLab, BitBucket and AzureDevOps providers.
// signingSecret is an optional string that is used to generate a SHA256 hmac of
// the notification body, which is sent along with the notification.
//
// TODO: This should accept something that can provide
// username/channel/token/signingSecret on demand.
func NewFactory(url, proxy, username, channel, token, signingSecret string) *Factory {
	return &Factory{
		URL:           url,
		ProxyURL:      proxy,
		Channel:       channel,
		Username:      username,
		Token:         token,
		SigningSecret: signingSecret,
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
		n, err = NewForwarder(f.URL, f.ProxyURL, f.SigningSecret)
	case v1beta1.SlackProvider:
		n, err = NewSlack(f.URL, f.ProxyURL, f.Username, f.Channel)
	case v1beta1.DiscordProvider:
		n, err = NewDiscord(f.URL, f.ProxyURL, f.Username, f.Channel)
	case v1beta1.RocketProvider:
		n, err = NewRocket(f.URL, f.ProxyURL, f.Username, f.Channel)
	case v1beta1.MSTeamsProvider:
		n, err = NewMSTeams(f.URL, f.ProxyURL)
	case v1beta1.GitHubProvider:
		n, err = NewGitHub(f.URL, f.Token)
	case v1beta1.GitLabProvider:
		n, err = NewGitLab(f.URL, f.Token)
	case v1beta1.BitbucketProvider:
		n, err = NewBitbucket(f.URL, f.Token)
	case v1beta1.AzureDevOpsProvider:
		n, err = NewAzureDevOps(f.URL, f.Token)
	case v1beta1.GoogleChatProvider:
		n, err = NewGoogleChat(f.URL, f.ProxyURL)
	case v1beta1.WebexProvider:
		n, err = NewWebex(f.URL, f.ProxyURL)
	case v1beta1.SentryProvider:
		n, err = NewSentry(f.URL)
	default:
		err = fmt.Errorf("provider %s not supported", provider)
	}

	if err != nil {
		n = &NopNotifier{}
	}
	return n, err
}
