module github.com/doitintl/gtoken/daemon

go 1.16

require (
	github.com/doitintl/gtoken/internal/aws v0.0.0-00010101000000-000000000000
	github.com/doitintl/gtoken/internal/gcp v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli/v2 v2.0.0
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
)

replace (
	github.com/doitintl/gtoken/internal/aws => ../internal/aws
	github.com/doitintl/gtoken/internal/gcp => ../internal/gcp
)
