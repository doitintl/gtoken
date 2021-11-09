package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/doitintl/gtoken/internal/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//nolint:funlen
func Test_generateIDToken(t *testing.T) {
	type args struct {
		file    string
		refresh bool
	}
	type fields struct {
		email string
		jwt   string
	}
	tests := []struct {
		name     string
		args     args
		fields   fields
		mockInit func(context.Context, *gcp.MockServiceAccountInfo, *gcp.MockToken, args, fields)
		wantErr  bool
	}{
		{
			name: "one time token generation",
			args: args{
				file: "jwt.token",
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return("whatever", nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(nil)
			},
		},
		{
			name: "one time token generation from email",
			args: args{
				file: "jwt.token",
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return("", errors.New("failed to get sa"))
				sa.On("GetEmail").Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return(fields.jwt, nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(nil)
			},
		},
		{
			name: "refresh token generation",
			args: args{
				file:    "jwt.token",
				refresh: true,
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return(fields.jwt, nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(nil)
				token.On("GetDuration", fields.jwt).Return(31*time.Second, nil)
				token.On("Generate", ctx, fields.email).Return(fields.jwt, nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(nil)
			},
		},
		{
			name: "failed to find sa",
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return("", errors.New("failed to get sa"))
				sa.On("GetEmail").Return("", errors.New("failed to get sa email"))
			},
			wantErr: true,
		},
		{
			name: "failed to generate token",
			args: args{
				file: "jwt.token",
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return(fields.jwt, nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(errors.New("failed to write token to file"))
			},
			wantErr: true,
		},
		{
			name: "failed to write token",
			args: args{
				file: "jwt.token",
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return("", errors.New("failed to generate ID token"))
			},
			wantErr: true,
		},
		{
			name: "failed to get duration from token",
			args: args{
				file:    "jwt.token",
				refresh: true,
			},
			fields: fields{
				email: "test@project.iam.gserviceaccount.com",
				jwt:   "whatever",
			},
			mockInit: func(ctx context.Context, sa *gcp.MockServiceAccountInfo, token *gcp.MockToken, args args, fields fields) {
				sa.On("GetID", ctx).Return(fields.email, nil)
				token.On("Generate", ctx, fields.email).Return(fields.jwt, nil)
				token.On("WriteToFile", fields.jwt, args.file).Return(nil)
				token.On("GetDuration", fields.jwt).Return(time.Duration(0), errors.New("failed to get duration"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSA := &gcp.MockServiceAccountInfo{}
			mockToken := &gcp.MockToken{}
			ctx, cancel := context.WithCancel(context.TODO())
			tt.mockInit(ctx, mockSA, mockToken, tt.args, tt.fields)
			go func() {
				time.Sleep(time.Second)
				cancel()
			}()
			if err := generateIDToken(ctx, mockSA, mockToken, tt.args.file, tt.args.refresh); (err != nil) != tt.wantErr {
				t.Errorf("generateIDToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			mockSA.AssertExpectations(t)
			mockToken.AssertExpectations(t)
		})
	}
}

func Test_generateIDTokenCmd(t *testing.T) {
	ctx := context.Background()
	mockSA := &gcp.MockServiceAccountInfo{}
	mockToken := &gcp.MockToken{}
	fileName := "jwt.token"
	email := "test@project.iam.gserviceaccount.com"
	mockSA.On("GetID", mock.AnythingOfType("*context.cancelCtx")).Return(email, nil)
	mockToken.On("Generate", mock.AnythingOfType("*context.cancelCtx"), email).Return("whatever", nil)
	mockToken.On("WriteToFile", "whatever", fileName).Return(nil)
	mockToken.On("GetDuration", "whatever").Return(31*time.Second, nil)
	mockToken.On("Generate", mock.AnythingOfType("*context.cancelCtx"), email).Return("whatever", nil)
	mockToken.On("WriteToFile", "whatever", fileName).Return(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := startServerAndGenerator(ctx, mockSA, mockToken, fileName, true)
		assert.Nil(t, err, "generateIDTokenCmd should not return an error when context is canceled")
		wg.Done()
	}()

	var err error
	var resp *http.Response
	for i := 0; i < 100; i++ { // Try to post 10 times to allow the goroutine to catch up
		resp, err = http.Post(fmt.Sprintf("http://localhost%s/quitquitquit", ServerAddr), "", bytes.NewReader([]byte(""))) //nolint
		if err == nil {
			break
		}
	}

	if err != nil {
		t.Errorf("shouldn't receive an error while posting to webserver: %s", err)
		return
	}

	defer resp.Body.Close()

	assert.Equal(t, resp.StatusCode, http.StatusOK, "request should return a 200")
	wg.Wait()
}
