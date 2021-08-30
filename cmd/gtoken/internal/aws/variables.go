package aws

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

/* #nosec */
const (
	// AWS Web Identity Token ENV
	AwsWebIdentityTokenFile = "AWS_WEB_IDENTITY_TOKEN_FILE"
	AwsRoleArn              = "AWS_ROLE_ARN"
	AwsRoleSessionName      = "AWS_ROLE_SESSION_NAME"
)

type AWSEnvVariables interface {
	Generate(string, string) ([]string, error)
}

type AWSVariables struct{}

func NewAWSEnvVariables() AWSEnvVariables {
	return &AWSVariables{}
}

func (AWSVariables) Generate(roleArn, tokenFilePath string) ([]string, error) {
	log.Println("generating new AWS variables")
	if strings.EqualFold(tokenFilePath, "") {
		return nil, fmt.Errorf("environment variable %s cannot be empty", AwsWebIdentityTokenFile)
	}
	if strings.EqualFold(roleArn, "") {
		return nil, fmt.Errorf("environment variable %s cannot be empty", AwsRoleArn)
	}
	roleSessionName := fmt.Sprintf("gtoken-webhook-%s", randomString(16))
	return []string{
		fmt.Sprintf("%s=%s", AwsRoleArn, roleArn),
		fmt.Sprintf("%s=%s", AwsRoleSessionName, roleSessionName),
		fmt.Sprintf("%s=%s", AwsWebIdentityTokenFile, tokenFilePath),
	}, nil

}

// randomString Generates a random string of a-z chars with len = l
func randomString(l int) string {
	rand.Seed(time.Now().UnixNano())
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randomInt(97, 122))
	}
	return string(bytes)
}

// randomInt Returns an int >= min, < max
func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}
