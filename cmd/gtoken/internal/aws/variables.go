package aws

import (
	"fmt"
	"io"
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

type FileVariables interface {
	GenerateToFile(string) error
}

type EnvFileVariables struct{}

func NewAWSEnvFileVariables() FileVariables {
	return &EnvFileVariables{}
}

func (EnvFileVariables) GenerateToFile(fileName string) error {
	roleArn, ok := os.LookupEnv(awsRoleArn)
	if !ok {
		log.Fatalf("could not read from environment variable %s", awsRoleArn)
	}
	tokenFilePath, ok := os.LookupEnv(awsWebIdentityTokenFile)
	if !ok {
		log.Fatalf("could not read from environment variable %s", awsWebIdentityTokenFile)
	}
	log.Println("generating new AWS variables")
	roleSessionName := fmt.Sprintf("gtoken-webhook-%s", randomString(16))
	contents := fmt.Sprintf("export %s=%s\nexport %s=%s\nexport %s=%s\n",
		awsRoleSessionName, roleSessionName,
		awsWebIdentityTokenFile, tokenFilePath,
		awsRoleArn, roleArn)

	// this is a slice of io.Writers we will write the file to
	var writers []io.Writer

	// if DestFile was provided, lets try to create it and add to the writers
	if len(fileName) > 0 {
		file, err := os.Create(fileName)
		if err != nil {
			return fmt.Errorf("failed to create variables file: %s; error: %s", fileName, err.Error())
		}
		writers = append(writers, file)
		defer file.Close()
	}
	// MultiWriter(io.Writer...) returns a single writer which multiplexes its
	// writes across all of the writers we pass in.
	dest := io.MultiWriter(writers...)
	// write to dest the same way as before, copying from the Body
	if _, err := io.WriteString(dest, contents); err != nil {
		return fmt.Errorf("failed to write variables file: %s", err.Error())
	}
	return nil
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
