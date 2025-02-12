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
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"hash"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v64/github"
	"github.com/google/uuid"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/logger"

	apiv1 "github.com/fluxcd/notification-controller/api/v1"
)

func Test_handlePayload(t *testing.T) {
	type hashOpts struct {
		calculate func() hash.Hash
		prefix    string
		header    string
	}

	testReceiverResource := &apiv1.Receiver{
		TypeMeta: metav1.TypeMeta{
			Kind:       apiv1.ReceiverKind,
			APIVersion: apiv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource",
		},
	}

	testSecretWithToken := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "token",
		},
		Data: map[string][]byte{
			"token": []byte("token"),
		},
	}

	largePayload := make(map[string]any)
	for keySize := len(uuid.NewString()); len(largePayload)*keySize <= maxRequestSizeBytes; {
		largePayload[uuid.NewString()] = "something"
	}

	tests := []struct {
		name                       string
		hashOpts                   hashOpts
		headers                    map[string]string
		payload                    map[string]any
		receiver                   *apiv1.Receiver
		receiverType               string
		secret                     *corev1.Secret
		resources                  []client.Object
		noCrossNamespaceRefs       bool
		expectedResourcesAnnotated int
		expectedResponseCode       int
	}{
		{
			name: "Generic receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "gitlab receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gitlab-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GitLabReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			headers: map[string]string{
				"X-Gitlab-Token": "token",
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "cdevents receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdevents-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type:   apiv1.CDEventsReceiver,
					Events: []string{"cd.change.merged.v1"},
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			headers: map[string]string{
				"Ce-Type": "cd.change.merged.v1",
			},
			payload: map[string]any{
				"context": map[string]string{
					"gitRepository": "adamkenihan/notification-controller",
					"gitRevision":   "5555",
					"version":       "0.3.0",
					"id":            "5555",
					"source":        "github",
					"timestamp":     "2023-12-07T14:51:29.908479495Z",
					"type":          "dev.cdevents.change.merged.0.2.0",
				},
				"subject": map[string]string{
					"type": "change",
					"id":   "5555",
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "cdevents receiver wrong event type",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdevents-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type:   apiv1.CDEventsReceiver,
					Events: []string{"cd.environment.modified.v1"},
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			headers: map[string]string{
				"Ce-Type": "cd.change.merged.v1",
			},
			payload: map[string]any{
				"context": map[string]string{
					"gitRepository": "adamkenihan/notification-controller",
					"gitRevision":   "5555",
					"version":       "0.3.0",
					"id":            "5555",
					"source":        "github",
					"timestamp":     "2023-12-07T14:51:29.908479495Z",
					"type":          "dev.cdevents.change.merged.0.2.0",
				},
				"subject": map[string]string{
					"type": "change",
					"id":   "5555",
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name: "cdevents receiver no event type specified",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdevents-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.CDEventsReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			headers: map[string]string{
				"Ce-Type": "cd.change.merged.v1",
			},
			payload: map[string]any{
				"context": map[string]string{
					"gitRepository": "adamkenihan/notification-controller",
					"gitRevision":   "5555",
					"version":       "0.3.0",
					"id":            "5555",
					"source":        "github",
					"timestamp":     "2023-12-07T14:51:29.908479495Z",
					"type":          "dev.cdevents.change.merged.0.2.0",
				},
				"subject": map[string]string{
					"type": "change",
					"id":   "5555",
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "github receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GitHubReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			hashOpts: hashOpts{
				calculate: sha256.New,
				prefix:    "sha256=",
				header:    github.SHA256SignatureHeader,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]any{
				"action": "push",
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "generic hmac receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "generic-hmac-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericHMACReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			hashOpts: hashOpts{
				calculate: sha256.New,
				prefix:    "sha256=",
				header:    "X-Signature",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "bitbucket receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bitbucket-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type:   apiv1.BitbucketReceiver,
					Events: []string{"push"},
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			hashOpts: hashOpts{
				calculate: sha256.New,
				prefix:    "sha256=",
				header:    github.SHA256SignatureHeader,
			},
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Event-Key":  "push",
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "quay receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "quay-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.QuayReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			payload: map[string]any{
				"docker_url": "docker.io",
				"updated_tags": []string{
					"v0.0.1",
				},
			},
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "harbor receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "harbor-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.HarborReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			headers: map[string]string{
				"Authorization": "token",
			},
			expectedResponseCode: http.StatusOK,
		},
		{
			name: "missing secret",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "missing-secret",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "non-existing",
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name:                 "no receiver configured",
			expectedResponseCode: http.StatusNotFound,
		},
		{
			name: "not ready receiver is ignored",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "notready-receiver",
				},
				Spec: apiv1.ReceiverSpec{},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.StalledCondition, Status: metav1.ConditionFalse}},
				},
			},
			expectedResponseCode: http.StatusServiceUnavailable,
		},
		{
			name: "suspended receiver ignored",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "suspended-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Suspend: true,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			expectedResponseCode: http.StatusServiceUnavailable,
		},
		{
			name: "missing apiVersion in resource",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							Kind: apiv1.ReceiverKind,
							MatchLabels: map[string]string{
								"label": "match",
							},
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusInternalServerError,
		},
		{
			name: "resource by name not found",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "does-not-exists",
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusInternalServerError,
		},
		{
			name: "annotating resources by label match",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "*",
							MatchLabels: map[string]string{
								"label": "match",
							},
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource-2",
						Labels: map[string]string{
							"label": "does-not-match",
						},
					},
				},
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource",
						Labels: map[string]string{
							"label": "match",
						},
					},
				},
			},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "annotating resource by name",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "dummy-resource",
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource-2",
					},
				},
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource",
					},
				},
			},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "cannot annotate all resources if name is * and no labels were set",
			receiver: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "*",
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:               testSecretWithToken,
			expectedResponseCode: http.StatusInternalServerError,
		},
		{
			name: "no cross-namespace refs",
			receiver: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "some-resource",
							Namespace:  "another-namespace",
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
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
			noCrossNamespaceRefs: true,
			expectedResponseCode: http.StatusInternalServerError,
		},
		{
			name:    "large payload",
			payload: largePayload,
			receiver: &apiv1.Receiver{
				TypeMeta: metav1.TypeMeta{
					Kind:       apiv1.ReceiverKind,
					APIVersion: apiv1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "some-resource",
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			expectedResponseCode: http.StatusBadRequest,
		},
		{
			name: "resource matchLabels is ignored if name is not *",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "dummy-resource",
							MatchLabels: map[string]string{
								"label": "match",
							},
						},
					},
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource-2",
						Labels: map[string]string{
							"label": "match",
						},
					},
				},
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy-resource",
					},
				},
			},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "resources filtered with CEL expressions",
			headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			payload: map[string]any{
				"action": "INSERT",
				"digest": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world@sha256:6ec128e26cd5...",
				"tag":    "us-east1-docker.pkg.dev/my-project/my-repo/hello-world:1.1",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "*",
							MatchLabels: map[string]string{
								"label": "production",
							},
						},
					},
					ResourceFilter: `has(res.metadata.annotations) && req.tag.split('/').last().value().split(":").first().value() == res.metadata.annotations['update-image']`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-resource-1",
						Annotations: map[string]string{
							"update-image": "hello-world",
						},
						Labels: map[string]string{
							"label": "production",
						},
					},
				},
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-resource-2",
						Namespace: "tested",
						Labels: map[string]string{
							"label": "production",
						},
						Annotations: map[string]string{
							"update-image": "other-image",
						},
					},
				},
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-resource-3",
						Labels: map[string]string{
							"label": "production",
						},
					},
				},
			},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "filtering out a single named resource with CEL",
			headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			payload: map[string]any{
				"action": "INSERT",
				"digest": "us-east1-docker.pkg.dev/my-project/my-repo/hello-world@sha256:6ec128e26cd5...",
				"tag":    "us-east1-docker.pkg.dev/my-project/my-repo/hello-world:1.1",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `has(res.metadata.annotations) && req.tag.split('/').last().value().split(":").first().value() == res.metadata.annotations['update-image']`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-resource",
						Annotations: map[string]string{
							"update-image": "not-hello-world",
						},
					},
				},
			},
			expectedResourcesAnnotated: 0,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a GitHub receiver",
			hashOpts: hashOpts{
				calculate: sha256.New,
				header:    github.SHA256SignatureHeader,
				prefix:    "sha256=",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]any{
				"action": "push",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GitHubReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.action == 'push'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a GitLab receiver - (does not read body)",
			headers: map[string]string{
				"Content-Type":   "application/json",
				"X-Gitlab-Token": "token",
			},
			payload: map[string]any{
				"action": "push",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GitLabReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.action == 'push'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a cdevents receiver",
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cdevents-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type:   apiv1.CDEventsReceiver,
					Events: []string{"cd.change.merged.v1"},
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.context.gitRepository == 'adamkenihan/notification-controller'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			headers: map[string]string{
				"Ce-Type":      "cd.change.merged.v1",
				"Content-Type": "application/json; charset=utf-8",
			},
			payload: map[string]any{
				"context": map[string]string{
					"gitRepository": "adamkenihan/notification-controller",
					"gitRevision":   "5555",
					"version":       "0.3.0",
					"id":            "5555",
					"source":        "github",
					"timestamp":     "2023-12-07T14:51:29.908479495Z",
					"type":          "dev.cdevents.change.merged.0.2.0",
				},
				"subject": map[string]string{
					"type": "change",
					"id":   "5555",
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a Bitbucket receiver",
			hashOpts: hashOpts{
				calculate: sha256.New,
				header:    github.SHA256SignatureHeader,
				prefix:    "sha256=",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
				"X-Event-Key":  "push",
			},
			payload: map[string]any{
				"action": "push",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.BitbucketReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.action == 'push'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a quay receiver",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "quay-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.QuayReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.docker_url == 'docker.io'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			payload: map[string]any{
				"docker_url": "docker.io",
				"updated_tags": []string{
					"v0.0.1",
				},
			},
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a Harbor receiver - (does not read body)",
			headers: map[string]string{
				"Authorization": "token",
				"Content-Type":  "application/json",
			},
			payload: map[string]any{
				"event_type": "pushImage",
				"events": []map[string]any{
					{
						"project":      "prj",
						"repo_name":    "repo1",
						"tag":          "latest",
						"full_name":    "prj/repo1",
						"trigger_time": 158322233213,
						"image_id":     "9e2c9d5f44efbb6ee83aecd17a120c513047d289d142ec5738c9f02f9b24ad07",
						"project_type": "Private",
					},
				},
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "harbor-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.HarborReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.events[0].full_name == 'prj/repo1'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a Docker hub receiver",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]any{
				"push_data": map[string]any{
					"tag": "test-org/test-repo:v1.0",
				},
				"repository": map[string]any{
					"repo_url": "docker.io",
				},
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dockerhub-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.DockerHubReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.push_data.tag.split('/').last().value().split(':').first().value() == 'test-repo'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a GCR receiver",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer token",
			},
			payload: map[string]any{
				"message": map[string]any{
					"data": marshalGCRData(t, map[string]any{
						"action": "INSERT",
						"digest": "us-east1-docker.pkg.dev/my-project/my-repo/app1@sha256:6ec128e26cd5...",
						"tag":    "us-east1-docker.pkg.dev/my-project/my-repo/app1:v1.2.3",
					}),
				},
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gcr-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GCRReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.tag.split('/').last().value().split(":").first().value() == 'app1'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering a Nexus receiver",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]any{
				"timestamp":      "2016-11-10T23:57:49.664+0000",
				"nodeId":         "52905B51-085CCABB-CEBBEAAD-16795588-FC927D93",
				"initiator":      "admin/127.0.0.1",
				"repositoryName": "npm-proxy",
				"action":         "CREATED",
				"asset": map[string]any{
					"id":      "31c950c8eeeab78336308177ae9c441c",
					"assetId": "bnBtLXByb3h5OjMxYzk1MGM4ZWVlYWI3ODMzNjMwODE3N2FlOWM0NDFj",
					"format":  "npm",
					"name":    "concrete",
				},
			},
			hashOpts: hashOpts{
				calculate: sha1.New,
				header:    "X-Nexus-Webhook-Signature",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nexus-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.NexusReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.repositoryName == 'npm-proxy'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "CEL filtering an ACR receiver",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			payload: map[string]any{
				"action": "push",
				"target": map[string]any{
					"repository": "hello-world",
					"tag":        "v1",
				},
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nexus-receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.ACRReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "test-resource",
						},
					},
					ResourceFilter: `res.metadata.name == 'test-resource' && req.target.repository == 'hello-world'`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret:                     testSecretWithToken,
			resources:                  []client.Object{testReceiverResource},
			expectedResourcesAnnotated: 1,
			expectedResponseCode:       http.StatusOK,
		},
		{
			name: "handling errors when parsing the CEL expression results",
			headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			receiver: &apiv1.Receiver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "receiver",
				},
				Spec: apiv1.ReceiverSpec{
					Type: apiv1.GenericReceiver,
					SecretRef: meta.LocalObjectReference{
						Name: "token",
					},
					Resources: []apiv1.CrossNamespaceObjectReference{
						{
							APIVersion: apiv1.GroupVersion.String(),
							Kind:       apiv1.ReceiverKind,
							Name:       "*",
							MatchLabels: map[string]string{
								"label": "production",
							},
						},
					},
					ResourceFilter: `res.name == "test-resource-1"`,
				},
				Status: apiv1.ReceiverStatus{
					WebhookPath: apiv1.ReceiverWebhookPath,
					Conditions:  []metav1.Condition{{Type: meta.ReadyCondition, Status: metav1.ConditionTrue}},
				},
			},
			secret: testSecretWithToken,
			resources: []client.Object{
				&apiv1.Receiver{
					TypeMeta: metav1.TypeMeta{
						Kind:       apiv1.ReceiverKind,
						APIVersion: apiv1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-resource-1",
						Labels: map[string]string{
							"label": "production",
						},
					},
				},
			},
			expectedResourcesAnnotated: 0,
			expectedResponseCode:       http.StatusInternalServerError,
		},
	}

	scheme := runtime.NewScheme()
	apiv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			builder := fake.NewClientBuilder()
			builder.WithScheme(scheme)

			if tt.receiver != nil {
				builder.WithObjects(tt.receiver)
			}

			builder.WithObjects(tt.resources...)
			builder.WithIndex(&apiv1.Receiver{}, WebhookPathIndexKey, IndexReceiverWebhookPath)

			if tt.secret != nil {
				builder.WithObjects(tt.secret)
			}

			client := builder.Build()
			s := ReceiverServer{
				port:                 "",
				logger:               logger.NewLogger(logger.Options{}),
				kubeClient:           client,
				noCrossNamespaceRefs: tt.noCrossNamespaceRefs,
			}

			data, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("error marshalling test payload: '%s'", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/hook/", bytes.NewBuffer(data))
			for key, val := range tt.headers {
				req.Header.Set(key, val)
			}
			if tt.hashOpts.calculate != nil {
				mac := hmac.New(tt.hashOpts.calculate, tt.secret.Data["token"])
				_, err := mac.Write(data)
				if err != nil {
					t.Fatalf("error writing hmac: '%s'", err)
				}
				req.Header.Set(tt.hashOpts.header, tt.hashOpts.prefix+hex.EncodeToString(mac.Sum(nil)))
			}
			rr := httptest.NewRecorder()
			s.handlePayload(rr, req)
			g.Expect(rr.Result().StatusCode).To(gomega.Equal(tt.expectedResponseCode))

			var allReceivers apiv1.ReceiverList
			g.Expect(client.List(context.TODO(), &allReceivers)).To(gomega.Succeed())

			var annotatedResources int
			for _, obj := range allReceivers.Items {
				if _, ok := obj.GetAnnotations()[meta.ReconcileRequestAnnotation]; ok {
					annotatedResources++
				}
			}

			g.Expect(annotatedResources).To(gomega.Equal(tt.expectedResourcesAnnotated))
		})
	}
}

func buildTestClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	apiv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithIndex(&apiv1.Receiver{}, WebhookPathIndexKey, IndexReceiverWebhookPath).Build()
}

func marshalGCRData(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}
