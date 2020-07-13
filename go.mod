module github.com/fluxcd/notification-controller

go 1.13

require (
	github.com/fluxcd/pkg v0.0.2
	github.com/fluxcd/source-controller v0.0.2
	github.com/go-logr/logr v0.1.0
	github.com/google/go-github/v32 v32.0.0
	github.com/hashicorp/go-retryablehttp v0.6.6
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.10.0
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.4
	k8s.io/client-go v0.18.4
	sigs.k8s.io/controller-runtime v0.6.0
)
