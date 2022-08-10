/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/go-github/v41/github"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/notification-controller/api/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/logger"
)

func Test_validate(t *testing.T) {
	type hashOpts struct {
		calculate bool
		header    string
	}

	tests := []struct {
		name         string
		hashOpts     hashOpts
		headers      map[string]string
		payload      map[string]interface{}
		receiver     *v1beta1.Receiver
		receiverType string
		secret       *corev1.Secret
		expectedErr  bool
	}{
		{
			name: "Generic receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			expectedErr: false,
		},
		{
			name: "gitlab receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gitlab-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.GitLabReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			headers: map[string]string{
				"X-Gitlab-Token": "token",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			expectedErr: false,
		},
		{
			name: "github receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.GitHubReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			hashOpts: hashOpts{
				calculate: true,
				header:    github.SHA256SignatureHeader,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]interface{}{
				"action": "push",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			expectedErr: false,
		},
		{
			name: "generic hmac receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "generic-hmac-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.GenericHMACReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			hashOpts: hashOpts{
				calculate: true,
				header:    "X-Signature",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			expectedErr: false,
		},
		{
			name: "bitbucket receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bitbucket-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type:   v1beta1.BitbucketReceiver,
					Events: []string{"push"},
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			hashOpts: hashOpts{
				calculate: true,
				header:    github.SHA256SignatureHeader,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Event-Key":  "push",
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			expectedErr: false,
		},
		{
			name: "quay receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "quay-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.QuayReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			payload: map[string]interface{}{
				"docker_url": "docker.io",
				"updated_tags": []string{
					"v0.0.1",
				},
			},
			expectedErr: false,
		},
		{
			name: "harbor receiver",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "harbor-receiver",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.HarborReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
				Data: map[string][]byte{
					"token": []byte("token"),
				},
			},
			headers: map[string]string{
				"Authorization": "token",
			},
			expectedErr: false,
		},
		{
			name: "missing secret",
			receiver: &v1beta1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "missing-secret",
				},
				Spec: v1beta1.ReceiverSpec{
					Type: v1beta1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "non-existing",
					},
				},
			},
			expectedErr: true,
		},
	}

	scheme := runtime.NewScheme()
	v1beta1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			builder := fake.NewClientBuilder()
			builder.WithScheme(scheme)
			builder.WithObjects(tt.receiver)
			if tt.secret != nil {
				builder.WithObjects(tt.secret)
			}

			client := builder.Build()
			s := ReceiverServer{
				port:       "",
				logger:     logger.NewLogger(logger.Options{}),
				kubeClient: client,
			}

			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Errorf("error marshalling test payload: '%s'", err)
			}
			req := httptest.NewRequest("POST", "/", bytes.NewBuffer(data))
			for key, val := range tt.headers {
				req.Header.Set(key, val)
			}
			if tt.hashOpts.calculate {
				mac := hmac.New(sha256.New, tt.secret.Data["token"])
				_, err := mac.Write(data)
				if err != nil {
					t.Errorf("error writing hmac: '%s'", err)
				}
				req.Header.Set(tt.hashOpts.header, "sha256="+hex.EncodeToString(mac.Sum(nil)))
			}

			err = s.validate(context.Background(), *tt.receiver, req)
			if tt.expectedErr && err == nil {
				t.Errorf("expected error but got %s", err)
			}

			if !tt.expectedErr && err != nil {
				t.Errorf("unexpected error: '%s'", err)
			}
		})
	}
}
