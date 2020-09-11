module github.com/fluxcd/notification-controller

go 1.14

replace github.com/fluxcd/notification-controller/api => ./api

require (
	github.com/fluxcd/notification-controller/api v0.0.0-00010101000000-000000000000
	github.com/fluxcd/pkg/recorder v0.0.6
	github.com/fluxcd/pkg/runtime v0.0.1
	github.com/fluxcd/source-controller/api v0.0.13
	github.com/go-logr/logr v0.1.0
	github.com/google/go-github/v32 v32.0.0
	github.com/hashicorp/go-retryablehttp v0.6.6
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/stretchr/testify v1.6.1
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	sigs.k8s.io/controller-runtime v0.6.2
)
