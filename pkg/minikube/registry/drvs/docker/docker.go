/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package docker

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/golang/glog"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/registry"
)

func init() {
	priority := registry.Default
	// Staged rollout for preferred:
	// - Linux
	// - Windows (once "service" command works)
	// - macOS
	if runtime.GOOS == "linux" {
		priority = registry.Preferred
	}

	if err := registry.Register(registry.DriverDef{
		Name:     driver.Docker,
		Config:   configure,
		Init:     func() drivers.Driver { return kic.NewDriver(kic.Config{OCIBinary: oci.Docker}) },
		Status:   status,
		Priority: priority,
	}); err != nil {
		panic(fmt.Sprintf("register failed: %v", err))
	}
}

func configure(cc config.ClusterConfig, n config.Node) (interface{}, error) {
	return kic.NewDriver(kic.Config{
		MachineName:       driver.MachineName(cc, n),
		StorePath:         localpath.MiniPath(),
		ImageDigest:       cc.KicBaseImage,
		CPU:               cc.CPUs,
		Memory:            cc.Memory,
		OCIBinary:         oci.Docker,
		APIServerPort:     cc.Nodes[0].Port,
		KubernetesVersion: cc.KubernetesConfig.KubernetesVersion,
		ContainerRuntime:  cc.KubernetesConfig.ContainerRuntime,
	}), nil
}

func status() registry.State {
	docURL := "https://minikube.sigs.k8s.io/docs/drivers/docker/"
	if runtime.GOARCH != "amd64" {
		return registry.State{Error: fmt.Errorf("docker driver is not supported on %q systems yet", runtime.GOARCH), Installed: false, Healthy: false, Fix: "Try other drivers", Doc: docURL}
	}

	_, err := exec.LookPath(oci.Docker)
	if err != nil {
		return registry.State{Error: err, Installed: false, Healthy: false, Fix: "Install Docker", Doc: docURL}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Quickly returns an error code if server is not running
	cmd := exec.CommandContext(ctx, oci.Docker, "version", "--format", "{{.Server.Os}}-{{.Server.Version}}")
	o, err := cmd.Output()
	output := string(o)
	if strings.Contains(output, "windows-") {
		return registry.State{Error: oci.ErrWindowsContainers, Installed: true, Healthy: false, Fix: "Change container type to \"linux\" in Docker Desktop settings", Doc: docURL + "#verify-docker-container-type-is-linux"}

	}
	if err == nil {
		glog.Infof("docker version: %s", output)
		return registry.State{Installed: true, Healthy: true}
	}

	glog.Warningf("docker returned error: %v", err)

	// Basic timeout
	if ctx.Err() == context.DeadlineExceeded {
		return registry.State{Error: err, Installed: true, Healthy: false, Fix: "Restart the Docker service", Doc: docURL}
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		newErr := fmt.Errorf(`%q %v: %s`, strings.Join(cmd.Args, " "), exitErr, stderr)

		if strings.Contains(stderr, "permission denied") && runtime.GOOS == "linux" {
			return registry.State{Error: newErr, Installed: true, Healthy: false, Fix: "Add your user to the 'docker' group: 'sudo usermod -aG docker $USER && newgrp docker'", Doc: "https://docs.docker.com/engine/install/linux-postinstall/"}
		}

		if strings.Contains(stderr, "Cannot connect") || strings.Contains(stderr, "refused") || strings.Contains(stderr, "Is the docker daemon running") {
			return registry.State{Error: newErr, Installed: true, Healthy: false, Fix: "Start the Docker service", Doc: docURL}
		}

		// We don't have good advice, but at least we can provide a good error message
		return registry.State{Error: newErr, Installed: true, Healthy: false, Doc: docURL}
	}

	return registry.State{Error: err, Installed: true, Healthy: false, Doc: docURL}
}
