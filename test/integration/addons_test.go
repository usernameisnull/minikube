// +build integration

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"k8s.io/minikube/pkg/kapi"
	"k8s.io/minikube/pkg/util/retry"
)

// TestAddons tests addons that require no special environment -- in parallel
func TestAddons(t *testing.T) {
	profile := UniqueProfileName("addons")
	ctx, cancel := context.WithTimeout(context.Background(), Minutes(40))
	defer Cleanup(t, profile, cancel)

	args := append([]string{"start", "-p", profile, "--wait=false", "--memory=2600", "--alsologtostderr", "--addons=ingress", "--addons=registry", "--addons=metrics-server", "--addons=helm-tiller", "--addons=olm"}, StartArgs()...)
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("%s failed: %v", rr.Command(), err)
	}

	// Parallelized tests
	t.Run("parallel", func(t *testing.T) {
		tests := []struct {
			name      string
			validator validateFunc
		}{
			{"Registry", validateRegistryAddon},
			{"Ingress", validateIngressAddon},
			{"MetricsServer", validateMetricsServerAddon},
			{"HelmTiller", validateHelmTillerAddon},
			{"Olm", validateOlmAddon},
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				MaybeParallel(t)
				tc.validator(ctx, t, profile)
			})
		}
	})

	// Assert that disable/enable works offline
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "stop", "-p", profile))
	if err != nil {
		t.Errorf("failed to stop minikube. args %q : %v", rr.Command(), err)
	}
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "addons", "enable", "dashboard", "-p", profile))
	if err != nil {
		t.Errorf("failed to enable dashboard addon: args %q : %v", rr.Command(), err)
	}
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "addons", "disable", "dashboard", "-p", profile))
	if err != nil {
		t.Errorf("failed to disable dashboard addon: args %q : %v", rr.Command(), err)
	}
}

func validateIngressAddon(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	if NoneDriver() {
		t.Skipf("skipping: ssh unsupported by none")
	}

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get Kubernetes client: %v", client)
	}

	if err := kapi.WaitForDeploymentToStabilize(client, "kube-system", "ingress-nginx-controller", Minutes(6)); err != nil {
		t.Errorf("failed waiting for ingress-controller deployment to stabilize: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "kube-system", "app.kubernetes.io/name=ingress-nginx", Minutes(12)); err != nil {
		t.Fatalf("failed waititing for nginx-ingress-controller : %v", err)
	}

	createIngress := func() error {
		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "replace", "--force", "-f", filepath.Join(*testdataDir, "nginx-ing.yaml")))
		if err != nil {
			return err
		}
		if rr.Stderr.String() != "" {
			t.Logf("%v: unexpected stderr: %s (may be temproary)", rr.Command(), rr.Stderr)
		}
		return nil
	}

	if err := retry.Expo(createIngress, 1*time.Second, Seconds(90)); err != nil {
		t.Errorf("failed to create ingress: %v", err)
	}

	rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "replace", "--force", "-f", filepath.Join(*testdataDir, "nginx-pod-svc.yaml")))
	if err != nil {
		t.Errorf("failed to kubectl replace nginx-pod-svc. args %q. %v", rr.Command(), err)
	}

	if _, err := PodWait(ctx, t, profile, "default", "run=nginx", Minutes(4)); err != nil {
		t.Fatalf("failed waiting for ngnix pod: %v", err)
	}
	if err := kapi.WaitForService(client, "default", "nginx", true, time.Millisecond*500, Minutes(10)); err != nil {
		t.Errorf("failed waiting for nginx service to be up: %v", err)
	}

	want := "Welcome to nginx!"
	addr := "http://127.0.0.1/"
	checkIngress := func() error {
		rr, err := Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "ssh", fmt.Sprintf("curl -s %s -H 'Host: nginx.example.com'", addr)))
		if err != nil {
			return err
		}

		stderr := rr.Stderr.String()
		if rr.Stderr.String() != "" {
			t.Logf("debug: unexpected stderr for %v:\n%s", rr.Command(), stderr)
		}

		stdout := rr.Stdout.String()
		if !strings.Contains(stdout, want) {
			return fmt.Errorf("%v stdout = %q, want %q", rr.Command(), stdout, want)
		}
		return nil
	}

	if err := retry.Expo(checkIngress, 500*time.Millisecond, Seconds(90)); err != nil {
		t.Errorf("failed to get expected response from %s within minikube: %v", addr, err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "addons", "disable", "ingress", "--alsologtostderr", "-v=1"))
	if err != nil {
		t.Errorf("failed to disable ingress addon. args %q : %v", rr.Command(), err)
	}
}

func validateRegistryAddon(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get Kubernetes client for %s : %v", profile, err)
	}

	start := time.Now()
	if err := kapi.WaitForRCToStabilize(client, "kube-system", "registry", Minutes(6)); err != nil {
		t.Errorf("failed waiting for registry replicacontroller to stabilize: %v", err)
	}
	t.Logf("registry stabilized in %s", time.Since(start))

	if _, err := PodWait(ctx, t, profile, "kube-system", "actual-registry=true", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for pod actual-registry: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "kube-system", "registry-proxy=true", Minutes(10)); err != nil {
		t.Fatalf("failed waiting for pod registry-proxy: %v", err)
	}

	// Test from inside the cluster (no curl available on busybox)
	rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "delete", "po", "-l", "run=registry-test", "--now"))
	if err != nil {
		t.Logf("pre-cleanup %s failed: %v (not a problem)", rr.Command(), err)
	}

	rr, err = Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "run", "--rm", "registry-test", "--restart=Never", "--image=busybox", "-it", "--", "sh", "-c", "wget --spider -S http://registry.kube-system.svc.cluster.local"))
	if err != nil {
		t.Errorf("failed to hit registry.kube-system.svc.cluster.local. args %q failed: %v", rr.Command(), err)
	}
	want := "HTTP/1.1 200"
	if !strings.Contains(rr.Stdout.String(), want) {
		t.Errorf("expected curl response be %q, but got *%s*", want, rr.Stdout.String())
	}

	if NeedsPortForward() {
		t.Skipf("Unable to complete rest of the test due to connectivity assumptions")
	}

	// Test from outside the cluster
	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "ip"))
	if err != nil {
		t.Fatalf("failed run minikube ip. args %q : %v", rr.Command(), err)
	}
	if rr.Stderr.String() != "" {
		t.Errorf("expected stderr to be -empty- but got: *%q* .  args %q", rr.Stderr, rr.Command())
	}

	endpoint := fmt.Sprintf("http://%s:%d", strings.TrimSpace(rr.Stdout.String()), 5000)
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", endpoint, err)
	}

	checkExternalAccess := func() error {
		resp, err := retryablehttp.Get(u.String())
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s = status code %d, want %d", u, resp.StatusCode, http.StatusOK)
		}
		return nil
	}

	if err := retry.Expo(checkExternalAccess, 500*time.Millisecond, Seconds(150)); err != nil {
		t.Errorf("failed to check external access to %s: %v", u.String(), err.Error())
	}

	rr, err = Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "addons", "disable", "registry", "--alsologtostderr", "-v=1"))
	if err != nil {
		t.Errorf("failed to disable registry addon. args %q: %v", rr.Command(), err)
	}
}

func validateMetricsServerAddon(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get Kubernetes client for %s: %v", profile, err)
	}

	start := time.Now()
	if err := kapi.WaitForDeploymentToStabilize(client, "kube-system", "metrics-server", Minutes(6)); err != nil {
		t.Errorf("failed waiting for metrics-server deployment to stabilize: %v", err)
	}
	t.Logf("metrics-server stabilized in %s", time.Since(start))

	if _, err := PodWait(ctx, t, profile, "kube-system", "k8s-app=metrics-server", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for k8s-app=metrics-server pod: %v", err)
	}

	want := "CPU(cores)"
	checkMetricsServer := func() error {
		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "top", "pods", "-n", "kube-system"))
		if err != nil {
			return err
		}
		if rr.Stderr.String() != "" {
			t.Logf("%v: unexpected stderr: %s", rr.Command(), rr.Stderr)
		}
		if !strings.Contains(rr.Stdout.String(), want) {
			return fmt.Errorf("%v stdout = %q, want %q", rr.Command(), rr.Stdout, want)
		}
		return nil
	}

	// metrics-server takes some time to be able to collect metrics
	if err := retry.Expo(checkMetricsServer, time.Second*3, Minutes(6)); err != nil {
		t.Errorf("failed checking metric server: %v", err.Error())
	}

	rr, err := Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "addons", "disable", "metrics-server", "--alsologtostderr", "-v=1"))
	if err != nil {
		t.Errorf("failed to disable metrics-server addon: args %q: %v", rr.Command(), err)
	}
}

func validateHelmTillerAddon(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get Kubernetes client for %s: %v", profile, err)
	}

	start := time.Now()
	if err := kapi.WaitForDeploymentToStabilize(client, "kube-system", "tiller-deploy", Minutes(6)); err != nil {
		t.Errorf("failed waiting for tiller-deploy deployment to stabilize: %v", err)
	}
	t.Logf("tiller-deploy stabilized in %s", time.Since(start))

	if _, err := PodWait(ctx, t, profile, "kube-system", "app=helm", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for helm pod: %v", err)
	}

	if NoneDriver() {
		_, err := exec.LookPath("socat")
		if err != nil {
			t.Skipf("socat is required by kubectl to complete this test")
		}
	}

	want := "Server: &version.Version"
	// Test from inside the cluster (`helm version` use pod.list permission. we use tiller serviceaccount in kube-system to list pod)
	checkHelmTiller := func() error {

		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "run", "--rm", "helm-test", "--restart=Never", "--image=alpine/helm:2.16.3", "-it", "--namespace=kube-system", "--serviceaccount=tiller", "--", "version"))
		if err != nil {
			return err
		}
		if rr.Stderr.String() != "" {
			t.Logf("%v: unexpected stderr: %s", rr.Command(), rr.Stderr)
		}
		if !strings.Contains(rr.Stdout.String(), want) {
			return fmt.Errorf("%v stdout = %q, want %q", rr.Command(), rr.Stdout, want)
		}
		return nil
	}

	if err := retry.Expo(checkHelmTiller, 500*time.Millisecond, Minutes(2)); err != nil {
		t.Errorf("failed checking helm tiller: %v", err.Error())
	}

	rr, err := Run(t, exec.CommandContext(ctx, Target(), "-p", profile, "addons", "disable", "helm-tiller", "--alsologtostderr", "-v=1"))
	if err != nil {
		t.Errorf("failed disabling helm-tiller addon. arg %q.s %v", rr.Command(), err)
	}
}

func validateOlmAddon(ctx context.Context, t *testing.T, profile string) {
	if NoneDriver() {
		t.Skipf("Skipping none driver, olm addon is not supported on none driver")
	}
	defer PostMortemLogs(t, profile)

	client, err := kapi.Client(profile)
	if err != nil {
		t.Fatalf("failed to get Kubernetes client for %s: %v", profile, err)
	}

	start := time.Now()
	if err := kapi.WaitForDeploymentToStabilize(client, "olm", "catalog-operator", Minutes(6)); err != nil {
		t.Errorf("failed waiting for catalog-operator deployment to stabilize: %v", err)
	}
	t.Logf("catalog-operator stabilized in %s", time.Since(start))
	if err := kapi.WaitForDeploymentToStabilize(client, "olm", "olm-operator", Minutes(6)); err != nil {
		t.Errorf("failed waiting for olm-operator deployment to stabilize: %v", err)
	}
	t.Logf("olm-operator stabilized in %s", time.Since(start))
	if err := kapi.WaitForDeploymentToStabilize(client, "olm", "packageserver", Minutes(6)); err != nil {
		t.Errorf("failed waiting for packageserver deployment to stabilize: %v", err)
	}
	t.Logf("packageserver stabilized in %s", time.Since(start))

	if _, err := PodWait(ctx, t, profile, "olm", "app=catalog-operator", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for pod catalog-operator: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "olm", "app=olm-operator", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for pod olm-operator: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "olm", "app=packageserver", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for pod packageserver: %v", err)
	}
	if _, err := PodWait(ctx, t, profile, "olm", "olm.catalogSource=operatorhubio-catalog", Minutes(6)); err != nil {
		t.Fatalf("failed waiting for pod operatorhubio-catalog: %v", err)
	}

	// Install one sample Operator such as etcd
	rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "create", "-f", "https://operatorhub.io/install/etcd.yaml"))
	if err != nil {
		t.Logf("etcd operator installation with %s failed: %v", rr.Command(), err)
	}

	want := "Succeeded"
	checkOperatorInstalled := func() error {
		rr, err := Run(t, exec.CommandContext(ctx, "kubectl", "--context", profile, "get", "csv", "-n", "my-etcd"))
		if err != nil {
			return err
		}
		if rr.Stderr.String() != "" {
			t.Logf("%v: unexpected stderr: %s", rr.Command(), rr.Stderr)
		}
		if !strings.Contains(rr.Stdout.String(), want) {
			return fmt.Errorf("%v stdout = %q, want %q", rr.Command(), rr.Stdout, want)
		}
		return nil
	}

	// Operator installation takes a while
	if err := retry.Expo(checkOperatorInstalled, time.Second*3, Minutes(6)); err != nil {
		t.Errorf("failed checking operator installed: %v", err.Error())
	}
}
