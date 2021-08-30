package aws

import (
	"fmt"
	"regexp"
	"testing"
)

func TestAWSVariables_Generate(t *testing.T) {
	type args struct {
		roleArn       string
		tokenFilePath string
	}
	tests := []struct {
		name         string
		args         args
		wantPattern  []string
		wantErr      bool
		wantMatchErr bool
	}{
		{
			name: "Generate aws env vars",
			args: args{
				roleArn:       "arn:aws:iam::123456789012:role/S3Access",
				tokenFilePath: "/some/path/to/file",
			},
			wantPattern: []string{
				fmt.Sprintf("%s=%s", AwsRoleArn, "arn:aws:iam::123456789012:role/S3Access"),
				fmt.Sprintf("%s=%s", AwsRoleSessionName, "gtoken-webhook-\\w*"),
				fmt.Sprintf("%s=%s", AwsWebIdentityTokenFile, "/some/path/to/file"),
			},
		},
		{
			name: "Generate error (no file path provided)",
			args: args{
				roleArn: "arn:aws:iam::123456789012:role/S3Access",
			},
			wantErr: true,
		},
		{
			name: "Generate error (no arn provided)",
			args: args{
				tokenFilePath: "/path/to/file",
			},
			wantErr: true,
		},
		{
			name: "Match error",
			args: args{
				roleArn:       "arn:aws:iam::123456789012:role/S3Access",
				tokenFilePath: "/some/path/to/file",
			},
			wantPattern: []string{
				fmt.Sprintf("%s=%s", AwsRoleArn, "arn:aws:iam::123456789012:role/S3Access"),
				fmt.Sprintf("%s=%s", AwsRoleSessionName, "notamatch-\\w*"),
				fmt.Sprintf("%s=%s", AwsWebIdentityTokenFile, "/some/path/to/file"),
			},
			wantMatchErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aw := AWSVariables{}
			got, err := aw.Generate(tt.args.roleArn, tt.args.tokenFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for index, elem := range got {
				hasMatched, err2 := regexp.Match(tt.wantPattern[index], []byte(elem))
				if err2 != nil {
					t.Errorf("Generate() test error: could not compile regexp pattern: %v", err2)
					return
				}
				if !hasMatched && !tt.wantMatchErr {
					t.Errorf("Generate() test error: result is not a match for pattern! got = %v, pattern = %v", elem, tt.wantPattern[index])
					return
				}
			}
		})
	}
}
