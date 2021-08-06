module github.com/doitintl/gtoken-webhook

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/prometheus/client_golang v1.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/slok/kubewebhook v0.3.0
	github.com/urfave/cli v1.22.2
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6 // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/utils v0.0.0-20191218082557-f07c713de883 // indirect
	sigs.k8s.io/controller-runtime v0.4.0
)

replace golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20180821212333-d2e6202438be
