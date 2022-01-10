module github.com/fluxcd/notification-controller

go 1.16

replace github.com/fluxcd/notification-controller/api => ./api

require (
	github.com/Azure/azure-amqp-common-go/v3 v3.1.0
	github.com/Azure/azure-event-hubs-go/v3 v3.3.7
	github.com/Azure/azure-sdk-for-go v53.4.0+incompatible // indirect
	github.com/Azure/go-amqp v0.13.6 // indirect
	github.com/containrrr/shoutrrr v0.4.4
	github.com/fluxcd/notification-controller/api v0.18.1
	github.com/fluxcd/pkg/apis/meta v0.11.0-rc.1
	github.com/fluxcd/pkg/runtime v0.13.0-rc.5
	github.com/getsentry/sentry-go v0.11.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-github/v39 v39.0.0
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/ktrysmt/go-bitbucket v0.9.26
	github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/sethvargo/go-limiter v0.7.2
	github.com/slok/go-http-metrics v0.9.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/whilp/git-urls v1.0.0
	github.com/xanzy/go-gitlab v0.50.4
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.2
	sigs.k8s.io/yaml v1.2.0
)
