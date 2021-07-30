module github.com/doitintl/gtoken

go 1.16

require (
	github.com/doitintl/gtoken/internal/aws v1.0.0
	github.com/doitintl/gtoken/internal/gcp v1.0.0
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/urfave/cli/v2 v2.0.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect

)

replace (
	github.com/doitintl/gtoken/internal/aws => ./internal/aws
	github.com/doitintl/gtoken/internal/gcp => ./internal/gcp
)
