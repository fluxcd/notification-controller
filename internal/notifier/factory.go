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
	"fmt"
	"net/url"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"
	fluxbroker "github.com/fluxcd/pkg/spiffe/brokerapi"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

// notifiers is the registry of the constructors of the implemented notifiers.
var notifiers = map[string]func(ctx context.Context, opts *notifierOptions) (Interface, error){
	apiv1.GenericProvider:                   NewGeneric,
	apiv1.GenericHMACProvider:               NewGenericHMAC,
	apiv1.SlackProvider:                     NewSlack,
	apiv1.DiscordProvider:                   NewDiscord,
	apiv1.RocketProvider:                    NewRocket,
	apiv1.MSTeamsProvider:                   NewMSTeams,
	apiv1.GoogleChatProvider:                NewGoogleChat,
	apiv1.GooglePubSubProvider:              NewGooglePubSub,
	apiv1.WebexProvider:                     NewWebex,
	apiv1.SentryProvider:                    NewSentry,
	apiv1.AzureEventHubProvider:             NewAzureEventHub,
	apiv1.TelegramProvider:                  NewTelegram,
	apiv1.LarkProvider:                      NewLark,
	apiv1.Matrix:                            NewMatrix,
	apiv1.OpsgenieProvider:                  NewOpsgenie,
	apiv1.AlertManagerProvider:              NewAlertManager,
	apiv1.GrafanaProvider:                   NewGrafana,
	apiv1.PagerDutyProvider:                 NewPagerDuty,
	apiv1.DataDogProvider:                   NewDataDog,
	apiv1.NATSProvider:                      NewNATS,
	apiv1.GitHubProvider:                    NewGitHub,
	apiv1.GitHubDispatchProvider:            NewGitHubDispatch,
	apiv1.GitHubPullRequestCommentProvider:  NewGitHubPullRequestComment,
	apiv1.GitLabProvider:                    NewGitLab,
	apiv1.GitLabMergeRequestCommentProvider: NewGitLabMergeRequestComment,
	apiv1.GiteaProvider:                     NewGitea,
	apiv1.GiteaPullRequestCommentProvider:   NewGiteaPullRequestComment,
	apiv1.BitbucketServerProvider:           NewBitbucketServer,
	apiv1.BitbucketProvider:                 NewBitbucket,
	apiv1.AzureDevOpsProvider:               NewAzureDevOps,
	apiv1.ZulipProvider:                     NewZulip,
	apiv1.OTELProvider:                      NewOTEL,
	apiv1.ZoomProvider:                      NewZoom,
	apiv1.IncidentioProvider:                NewIncidentio,
	apiv1.MastodonProvider:                  NewMastodon,
}

type notifierOptions struct {
	URL                   string
	ProxyURL              *url.URL
	Username              string
	Channel               string
	Token                 string
	Timeout               time.Duration
	Headers               map[string]string
	TLSConfig             *tls.Config
	Password              string
	CommitStatus          string
	ProviderUID           string
	ChangeRequestComment  *apiv1.ChangeRequestComment
	ProviderName          string
	ProviderNamespace     string
	SecretData            map[string][]byte
	ServiceAccountName    string
	DefaultServiceAccount string
	TokenCache            *cache.TokenCache
	TokenClient           client.Client
	SPIFFEBroker          fluxbroker.Interface
	// EventKey is the identity of the event being dispatched, computed by
	// the event server. Notifiers may use it as an idempotency token.
	EventKey string
	// AlertMetadata is the metadata of the Alert that triggered the notification.
	AlertMetadata metav1.ObjectMeta
}

// Option represents a functional option for configuring a notifier.
type Option func(*notifierOptions)

// WithTimeout sets the provider timeout for clients without context support.
func WithTimeout(timeout time.Duration) Option {
	return func(o *notifierOptions) {
		o.Timeout = timeout
	}
}

// WithProxyURL sets the proxy URL for the notifier.
func WithProxyURL(proxyURL *url.URL) Option {
	return func(o *notifierOptions) {
		o.ProxyURL = proxyURL
	}
}

// WithEventKey sets the key identifying the event being dispatched.
func WithEventKey(key string) Option {
	return func(o *notifierOptions) {
		o.EventKey = key
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

// WithChangeRequestComment sets the change request comment configuration.
func WithChangeRequestComment(config *apiv1.ChangeRequestComment) Option {
	return func(o *notifierOptions) {
		o.ChangeRequestComment = config
	}
}

// WithAlertMetadata sets the metadata of the Alert that triggered the notification.
func WithAlertMetadata(metadata metav1.ObjectMeta) Option {
	return func(o *notifierOptions) {
		o.AlertMetadata = metadata
	}
}

// WithProviderUID sets the provider UID for the notifier.
func WithProviderUID(uid string) Option {
	return func(o *notifierOptions) {
		o.ProviderUID = uid
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

// WithSPIFFEBroker sets the client used to fetch JWT-SVIDs for the `spiffe`
// credential provider.
func WithSPIFFEBroker(client fluxbroker.Interface) Option {
	return func(o *notifierOptions) {
		o.SPIFFEBroker = client
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

// WithDefaultServiceAccount sets the default service account for the notifier,
// used when .spec.serviceAccountName is not specified.
func WithDefaultServiceAccount(serviceAccountName string) Option {
	return func(o *notifierOptions) {
		o.DefaultServiceAccount = serviceAccountName
	}
}

// WithURL sets the webhook URL for the notifier.
func WithURL(url string) Option {
	return func(o *notifierOptions) {
		o.URL = url
	}
}

// New creates a new notifier for the specified provider with optional configurations.
func New(ctx context.Context, provider string, opts ...Option) (Interface, error) {
	newNotifier, ok := notifiers[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not implemented", provider)
	}

	options := notifierOptions{}

	for _, opt := range opts {
		opt(&options)
	}

	return newNotifier(ctx, &options)
}
