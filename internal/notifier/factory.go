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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"

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
	Context  context.Context
	URL      string
	ProxyURL string
	Username string
	Channel  string
	Token    string
	Headers  map[string]string
	// CertPool is kept for Git platform providers (GitHub, GitLab, etc.) that use third-party SDKs.
	// TODO: Remove this field once all notifiers support client certificate authentication via TLSConfig.
	CertPool           *x509.CertPool
	TLSConfig          *tls.Config
	Password           string
	CommitStatus       string
	ProviderName       string
	ProviderNamespace  string
	SecretData         map[string][]byte
	ServiceAccountName string
	TokenCache         *cache.TokenCache
	TokenClient        client.Client
}

type Factory struct {
	notifierOptions
}

// Option represents a functional option for configuring a notifier.
type Option func(*notifierOptions)

// WithProxyURL sets the proxy URL for the notifier.
func WithProxyURL(url string) Option {
	return func(o *notifierOptions) {
		o.ProxyURL = url
	}
}

// WithUsername sets the username for the notifier.
func WithUsername(username string) Option {
	return func(o *notifierOptions) {
		o.Username = username
	}
}

// WithChannel sets the channel for the notifier.
func WithChannel(channel string) Option {
	return func(o *notifierOptions) {
		o.Channel = channel
	}
}

// WithToken sets the token for the notifier.
func WithToken(token string) Option {
	return func(o *notifierOptions) {
		o.Token = token
	}
}

// WithHeaders sets the headers for the notifier.
func WithHeaders(headers map[string]string) Option {
	return func(o *notifierOptions) {
		o.Headers = headers
	}
}

// WithCertPool sets the certificate pool for the notifier.
func WithCertPool(certPool *x509.CertPool) Option {
	return func(o *notifierOptions) {
		o.CertPool = certPool
	}
}

// WithTLSConfig sets the TLS configuration for the notifier.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(o *notifierOptions) {
		o.TLSConfig = tlsConfig
	}
}

// WithPassword sets the password for the notifier.
func WithPassword(password string) Option {
	return func(o *notifierOptions) {
		o.Password = password
	}
}

// WithCommitStatus sets the custom commit status for the notifier.
func WithCommitStatus(commitStatus string) Option {
	return func(o *notifierOptions) {
		o.CommitStatus = commitStatus
	}
}

// WithProviderName sets the provider name for the notifier.
func WithProviderName(name string) Option {
	return func(o *notifierOptions) {
		o.ProviderName = name
	}
}

// WithProviderNamespace sets the provider namespace for the notifier.
func WithProviderNamespace(namespace string) Option {
	return func(o *notifierOptions) {
		o.ProviderNamespace = namespace
	}
}

// WithSecretData sets the secret data for the notifier.
func WithSecretData(data map[string][]byte) Option {
	return func(o *notifierOptions) {
		o.SecretData = data
	}
}

// WithTokenCache sets the token cache for the notifier.
func WithTokenCache(cache *cache.TokenCache) Option {
	return func(o *notifierOptions) {
		o.TokenCache = cache
	}
}

// WithTokenClient sets the token client for the notifier.
func WithTokenClient(kubeClient client.Client) Option {
	return func(o *notifierOptions) {
		o.TokenClient = kubeClient
	}
}

// WithServiceAccount sets the service account for the notifier.
func WithServiceAccount(serviceAccountName string) Option {
	return func(o *notifierOptions) {
		o.ServiceAccountName = serviceAccountName
	}
}

// WithURL sets the webhook URL for the notifier.
func WithURL(url string) Option {
	return func(o *notifierOptions) {
		o.URL = url
	}
}

// NewFactory creates a new notifier factory with optional configurations.
func NewFactory(ctx context.Context, opts ...Option) *Factory {
	options := notifierOptions{
		Context: ctx,
	}

	for _, opt := range opts {
		opt(&options)
	}

	return &Factory{
		notifierOptions: options,
	}
}

func (f Factory) Notifier(provider string) (Interface, error) {
	notifier, ok := notifiers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not supported", provider)
	}

	return notifier(f.notifierOptions)
}

func genericNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewForwarder(opts.URL, opts.ProxyURL, opts.Headers, opts.TLSConfig, nil)
}

func genericHMACNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewForwarder(opts.URL, opts.ProxyURL, opts.Headers, opts.TLSConfig, []byte(opts.Token))
}

func slackNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewSlack(opts.URL, opts.ProxyURL, opts.Token, opts.TLSConfig, opts.Username, opts.Channel)
}

func discordNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewDiscord(opts.URL, opts.ProxyURL, opts.Username, opts.Channel)
}

func rocketNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewRocket(opts.URL, opts.ProxyURL, opts.TLSConfig, opts.Username, opts.Channel)
}

func msteamsNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewMSTeams(opts.URL, opts.ProxyURL, opts.TLSConfig)
}

func googleChatNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGoogleChat(opts.URL, opts.ProxyURL)
}

func googlePubSubNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGooglePubSub(opts.URL, opts.Channel, opts.Token, opts.Headers)
}

func webexNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewWebex(opts.URL, opts.ProxyURL, opts.TLSConfig, opts.Channel, opts.Token)
}

func sentryNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewSentry(opts.CertPool, opts.URL, opts.Channel)
}

func azureEventHubNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAzureEventHub(opts.Context, opts.URL, opts.Token, opts.Channel, opts.ProxyURL, opts.ServiceAccountName, opts.ProviderName, opts.ProviderNamespace, opts.TokenClient, opts.TokenCache)
}

func telegramNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewTelegram(opts.ProxyURL, opts.Channel, opts.Token)
}

func larkNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewLark(opts.URL)
}

func matrixNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewMatrix(opts.URL, opts.Token, opts.Channel, opts.TLSConfig)
}

func opsgenieNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewOpsgenie(opts.URL, opts.ProxyURL, opts.TLSConfig, opts.Token)
}

func alertmanagerNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAlertmanager(opts.URL, opts.ProxyURL, opts.TLSConfig, opts.Token)
}

func grafanaNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewGrafana(opts.URL, opts.ProxyURL, opts.Token, opts.TLSConfig, opts.Username, opts.Password)
}

func pagerDutyNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewPagerDuty(opts.URL, opts.ProxyURL, opts.TLSConfig, opts.Channel)
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
	return NewGitHub(opts.CommitStatus, opts.URL, opts.Token, opts.CertPool, opts.ProxyURL, opts.ProviderName, opts.ProviderNamespace, opts.SecretData, opts.TokenCache)
}

func gitHubDispatchNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitHubDispatch(opts.URL, opts.Token, opts.CertPool, opts.ProxyURL, opts.ProviderName, opts.ProviderNamespace, opts.SecretData, opts.TokenCache)
}

func gitLabNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitLab(opts.CommitStatus, opts.URL, opts.Token, opts.CertPool)
}

func giteaNotifierFunc(opts notifierOptions) (Interface, error) {
	if opts.Token == "" && opts.Password != "" {
		opts.Token = opts.Password
	}
	return NewGitea(opts.CommitStatus, opts.URL, opts.ProxyURL, opts.Token, opts.CertPool)
}

func bitbucketServerNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewBitbucketServer(opts.CommitStatus, opts.URL, opts.Token, opts.CertPool, opts.Username, opts.Password)
}

func bitbucketNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewBitbucket(opts.CommitStatus, opts.URL, opts.Token, opts.CertPool)
}

func azureDevOpsNotifierFunc(opts notifierOptions) (Interface, error) {
	return NewAzureDevOps(opts.CommitStatus, opts.URL, opts.Token, opts.CertPool)
}
