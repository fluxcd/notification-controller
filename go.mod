module github.com/fluxcd/notification-controller

go 1.15

replace github.com/fluxcd/notification-controller/api => ./api

require (
	github.com/fluxcd/notification-controller/api v0.2.1
	github.com/fluxcd/pkg/apis/meta v0.4.0
	github.com/fluxcd/pkg/recorder v0.0.6
	github.com/fluxcd/pkg/runtime v0.3.0
	github.com/fluxcd/source-controller/api v0.3.0
	github.com/go-logr/logr v0.2.1
	github.com/google/go-github/v32 v32.0.0
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/ktrysmt/go-bitbucket v0.6.5
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/stretchr/testify v1.6.1
	github.com/whilp/git-urls v1.0.0
	github.com/xanzy/go-gitlab v0.37.0
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	sigs.k8s.io/controller-runtime v0.6.3
)
