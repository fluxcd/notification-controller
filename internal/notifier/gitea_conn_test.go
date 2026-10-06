/*
Copyright 2026 The Flux authors

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

package notifier

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

// TestGitea_PostDoesNotLeakConnections creates one notifier per event, the
// way the event handler does, and checks that no idle connection to the Gitea
// server is left behind once the notifiers are gone.
func TestGitea_PostDoesNotLeakConnections(t *testing.T) {
	g := NewWithT(t)

	defer func(d time.Duration) { giteaIdleConnTimeout = d }(giteaIdleConnTimeout)
	giteaIdleConnTimeout = 100 * time.Millisecond

	var mu sync.Mutex
	open := map[net.Conn]struct{}{}
	srv := httptest.NewUnstartedServer(newGiteaStubHandler(t))
	srv.Config.ConnState = func(c net.Conn, s http.ConnState) {
		mu.Lock()
		defer mu.Unlock()
		switch s {
		case http.StateNew:
			open[c] = struct{}{}
		case http.StateClosed, http.StateHijacked:
			delete(open, c)
		}
	}
	srv.Start()
	defer srv.Close()

	openConns := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(open)
	}

	event := eventv1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Kustomization",
			Namespace: "flux-system",
			Name:      "podinfo-repo",
		},
		Severity:  "info",
		Timestamp: metav1.Time{Time: time.Now()},
		Metadata: map[string]string{
			eventv1.MetaRevisionKey: "main@sha1:69b59063470310ebbd88a9156325322a124e55a3",
		},
		Message: "Service/podinfo/podinfo configured",
	}

	const events = 20
	for i := 0; i < events; i++ {
		gitea, err := NewGitea("kustomization/gitops-system/0c9c2e41",
			WithGiteaAddress(srv.URL+"/foo/bar"),
			WithGiteaToken("foobar"),
		)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gitea.Post(context.TODO(), event)).To(Succeed())
	}

	g.Eventually(openConns, 5*time.Second, 50*time.Millisecond).Should(BeZero(),
		"idle connections to the Gitea server must not outlive their notifier")
}
