module github.com/fluxcd/notification-controller

go 1.20

replace github.com/fluxcd/notification-controller/api => ./api

require (
	cloud.google.com/go/pubsub v1.33.0
	code.gitea.io/sdk/gitea v0.15.1
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230811130428-ced1acdcaa24
	github.com/Azure/azure-amqp-common-go/v4 v4.2.0
	github.com/Azure/azure-event-hubs-go/v3 v3.6.1
	github.com/DataDog/datadog-api-client-go/v2 v2.15.0
	github.com/PagerDuty/go-pagerduty v1.7.0
	github.com/containrrr/shoutrrr v0.8.0
	github.com/fluxcd/cli-utils v0.36.0-flux.1
	github.com/fluxcd/notification-controller/api v1.1.0
	github.com/fluxcd/pkg/apis/event v0.6.0
	github.com/fluxcd/pkg/apis/meta v1.2.0
	github.com/fluxcd/pkg/git v0.15.0
	github.com/fluxcd/pkg/masktoken v0.2.0
	github.com/fluxcd/pkg/runtime v0.43.0
	github.com/fluxcd/pkg/ssa v0.34.0
	github.com/getsentry/sentry-go v0.23.0
	github.com/go-logr/logr v1.3.0
	github.com/google/go-github/v53 v53.2.0
	github.com/hashicorp/go-retryablehttp v0.7.5
	github.com/ktrysmt/go-bitbucket v0.9.66
	github.com/microsoft/azure-devops-go-api/azuredevops/v6 v6.0.1
	github.com/nats-io/nats.go v1.31.0
	github.com/onsi/gomega v1.30.0
	github.com/sethvargo/go-limiter v0.7.2
	github.com/slok/go-http-metrics v0.10.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/whilp/git-urls v1.0.0
	github.com/xanzy/go-gitlab v0.90.0
	golang.org/x/oauth2 v0.14.0
	golang.org/x/text v0.14.0
	google.golang.org/api v0.138.0
	k8s.io/api v0.28.4
	k8s.io/apimachinery v0.28.4
	k8s.io/client-go v0.28.4
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/controller-runtime v0.16.3
	sigs.k8s.io/yaml v1.4.0
)

// Fix CVE-2022-28948
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1

require (
	cloud.google.com/go v0.110.6 // indirect
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.1 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-amqp v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/DataDog/zstd v1.5.2 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20231012073058-a7379d079e0e // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/cloudflare/circl v1.3.3 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/devigned/tab v0.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.7.0 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/fluxcd/pkg/apis/acl v0.1.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.5 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.5 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nkeys v0.4.5 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.starlark.net v0.0.0-20230525235612-a134d8f9ddca // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/crypto v0.15.0 // indirect
	golang.org/x/exp v0.0.0-20220827204233-334a2380cb91 // indirect
	golang.org/x/net v0.18.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/term v0.14.0 // indirect
	golang.org/x/time v0.4.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230807174057-1744710a1577 // indirect
	google.golang.org/grpc v1.57.1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/evanphx/json-patch.v5 v5.6.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.4 // indirect
	k8s.io/cli-runtime v0.28.4 // indirect
	k8s.io/component-base v0.28.4 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20231113174909-778a5567bc1e // indirect
	k8s.io/kubectl v0.28.4 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.15.0 // indirect
	sigs.k8s.io/kustomize/kyaml v0.15.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)
