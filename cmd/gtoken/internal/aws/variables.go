package aws

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

/* #nosec */
const (
	// AWS Web Identity Token ENV
	awsWebIdentityTokenFile = "AWS_WEB_IDENTITY_TOKEN_FILE"
	awsRoleArn              = "AWS_ROLE_ARN"
	awsRoleSessionName      = "AWS_ROLE_SESSION_NAME"
)

type AWSEnvVariables interface {
	Generate() ([]string, error)
}

type AWSVariables struct{}

func NewAWSEnvVariables() AWSEnvVariables {
	return &AWSVariables{}
}

func (AWSVariables) Generate() ([]string, error) {
	roleArn, ok := os.LookupEnv(awsRoleArn)
	if !ok {
		return nil, fmt.Errorf("could not read from environment variable %s", awsRoleArn)
	}
	tokenFilePath, ok := os.LookupEnv(awsWebIdentityTokenFile)
	if !ok {
		return nil, fmt.Errorf("could not read from environment variable %s", awsWebIdentityTokenFile)
	}
	log.Println("generating new AWS variables")
	roleSessionName := fmt.Sprintf("gtoken-webhook-%s", randomString(16))
	return []string{
		fmt.Sprintf("%s=%s", awsRoleArn, roleArn),
		fmt.Sprintf("%s=%s", awsRoleSessionName, roleSessionName),
		fmt.Sprintf("%s=%s", awsWebIdentityTokenFile, tokenFilePath),
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
