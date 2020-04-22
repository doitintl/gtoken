package gcp

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
)

// code found on Chromium project, https://github.com/luci/luci-go/blob/master/auth/internal/gce.go
//
// A client with more relaxed timeouts compared to the default one, which was
// observed to timeout often on GKE when using Workload Identities.
var metadataClient = metadata.NewClient(&http.Client{
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		ResponseHeaderTimeout: 15 * time.Second, // default is 2
	},
})

type ServiceAccountInfo interface {
	GetEmail() (string, error)
	GetID(context.Context) (string, error)
}

type SaInfo struct{}

func NewSaInfo() ServiceAccountInfo {
	return &SaInfo{}
}

func (sa SaInfo) GetEmail() (string, error) {
	// use metadataClient (see above) instead of metadata
	// grab an email associated with the account. This must not be failing on
	// a healthy VM if the account is present. If it does, the metadata server isd broken.
	log.Println("getting email from metadata server")
	email, err := metadataClient.Email("")
	if err != nil {
		return "", errors.Wrap(err, "failed to get default email")
	}
	return email, nil
}

func (sa SaInfo) GetID(ctx context.Context) (string, error) {
	log.Println("getting service account")
	// handle the 'refresh token' command
	cx, cancel := context.WithCancel(ctx)
	// cancel current context on exit
	defer cancel()

	// Ensure the account has requested scopes. Assume 'cloud-platform' scope
	// covers all possible scopes. This is important when using GKE Workload
	// Identities: the metadata server always reports only 'cloud-platform' scope
	// there. Its presence should be enough to cover all scopes used in practice.
	// The exception is non-cloud scopes (like gerritcodereview or G Suite). To
	// use such scopes, one will have to use impersonation through Cloud IAM APIs,
	// which *are* covered by cloud-platform (see ActAsServiceAccount in auth.go).
	log.Println("getting scopes")
	availableScopes, err := metadataClient.Scopes("")
	if err != nil {
		log.Printf("failed to get available scopes: %s\n", err)
	}
	// print scopes and search for cloud-platform scope
	var found bool
	for _, s := range availableScopes {
		log.Printf("scope: %s", s)
		if strings.Contains(s, "cloud-platform") {
			found = true
		}
	}
	if !found {
		log.Println("appending cloud-platform scope")
		availableScopes = append(availableScopes, "https://www.googleapis.com/auth/cloud-platform")
	}
	log.Println("getting credentials")
	creds, err := google.FindDefaultCredentials(cx, availableScopes...)
	if err != nil {
		return "", errors.Wrap(err, "failed to find default credentials")
	}
	credsMap := make(map[string]interface{})
	err = json.Unmarshal(creds.JSON, &credsMap)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse credentials JSON")
	}
	if id, ok := credsMap["client_id"].(string); ok {
		return id, nil
	}
	return "", errors.New("failed to find service account ID")
}
