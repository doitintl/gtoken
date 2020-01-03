[![](https://github.com/doitintl/gtoken/workflows/Docker%20Image%20CI/badge.svg)](https://github.com/doitintl/gtoken/actions?query=workflow%3A"Docker+Image+CI") [![Docker Pulls](https://img.shields.io/docker/pulls/doitintl/gtoken.svg?style=popout)](https://hub.docker.com/r/doitintl/gtoken) [![](https://images.microbadger.com/badges/image/doitintl/gtoken.svg)](https://microbadger.com/images/doitintl/gtoken "Get your own image badge on microbadger.com")

# gtoken

The `gtoken` tool can get Google Cloud ID token when running with under GCP Service Account (for example, GKE Pod with Workload Identity).

## `gtoken` command syntax

```text
NAME:
   gtoken - generate ID token with current Google Cloud service account

USAGE:
   gtoken [global options] command [command options] [arguments...]

VERSION:
   v0.1.6-dirty

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --refresh      auto refresh ID token before it expires (default: true)
   --file value   write ID token into file (stdout, if not specified)
   --help, -h     show help (default: false)
   --version, -v  print the version
```

## AWS Web Identity Token

Within the `~/.aws/config` file, you can also configure a profile to indicate that AWS SDK should assume a role. When you do this, the AWS SDK will automatically make the corresponding `AssumeRoleWithWebIdentity` calls to AWS STS on your behalf. It will handle in memory caching as well as refreshing credentials as needed.

You can specify the following configuration values for configuring an IAM role:

- `role_arn` - The ARN of the role you want to assume.
- `web_identity_token_file` - The path to a file which contains an OAuth 2.0 access token or OpenID Connect ID token that is provided by the identity provider. The contents of this file will be loaded and passed as the `WebIdentityToken` argument to the `AssumeRoleWithWebIdentity` operation.
- `role_session_name` - The name applied to this assume-role session. This value affects the assumed role user ARN (such as `arn:aws:sts::123456789012:assumed-role/role_name/role_session_name`). This maps to the RoleSessionName parameter in the `AssumeRoleWithWebIdentity` operation. This is an optional parameter. If you do not provide this value, a session name will be automatically generated.

```text
# In ~/.aws/config
[profile web-identity]
role_arn=arn:aws:iam:...
web_identity_token_file=/path/to/a/token
```

This provider can also be configured via the environment:

`AWS_ROLE_ARN`
    The ARN of the role you want to assume.
`AWS_WEB_IDENTITY_TOKEN_FILE`
    The path to the web identity token file.
`AWS_ROLE_SESSION_NAME`
    The name applied to this assume-role session.

## mounting GCP id token as AWS Web Identity Token

Annotate Kubernetes Service Account with `amazonaws.com/role-arn=<AWS_ROLE_ARN>` annotation.
