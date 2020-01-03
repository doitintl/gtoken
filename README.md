[![](https://github.com/doitintl/gtoken/workflows/Docker%20Image%20CI/badge.svg)](https://github.com/doitintl/gtoken/actions?query=workflow%3A"Docker+Image+CI") [![Docker Pulls](https://img.shields.io/docker/pulls/doitintl/gtoken.svg?style=popout)](https://hub.docker.com/r/doitintl/gtoken) [![](https://images.microbadger.com/badges/image/doitintl/gtoken.svg)](https://microbadger.com/images/doitintl/gtoken "Get your own image badge on microbadger.com")

# gtoken

The `gtoken` tool can get Google Cloud ID token when running with under GCP Service Account (for example, GKE Pod with Workload Identity).

## command syntax

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