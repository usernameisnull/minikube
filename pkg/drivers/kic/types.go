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

package kic

import (
	"fmt"

	"k8s.io/minikube/pkg/drivers/kic/oci"
)

const (
	// Version is the current version of kic
	Version = "v0.0.10"
	// SHA of the kic base image
	baseImageSHA = "f58e0c4662bac8a9b5dda7984b185bad8502ade5d9fa364bf2755d636ab51438"
	// OverlayImage is the cni plugin used for overlay image, created by kind.
	// CNI plugin image used for kic drivers created by kind.
	OverlayImage = "kindest/kindnetd:0.5.4"
)

var (
	// BaseImage is the base image is used to spin up kic containers. it uses same base-image as kind.
	BaseImage = fmt.Sprintf("registry.cn-hangzhou.aliyuncs.com/google_containers/kicbase:%s@sha256:%s", Version, baseImageSHA)

	// BaseImageFallBack1 the fall back of BaseImage in case gcr.io is not available. stored in docker hub
	// same image is push to https://github.com/kicbase/stable
	BaseImageFallBack1 = fmt.Sprintf("kicbase/stable:%s@sha256:%s", Version, baseImageSHA)

	// BaseImageFallBack2 the fall back of BaseImage in case gcr.io is not available. stored in github packages https://github.com/kubernetes/minikube/packages/206071
	// github packages docker does _NOT_ support pulling by sha as mentioned in the docs:
	// https://help.github.com/en/packages/using-github-packages-with-your-projects-ecosystem/configuring-docker-for-use-with-github-packages
	BaseImageFallBack2 = fmt.Sprintf("docker.pkg.github.com/kubernetes/minikube/kicbase:%s", Version)
)

// Config is configuration for the kic driver used by registry
type Config struct {
	MachineName       string            // maps to the container name being created
	CPU               int               // Number of CPU cores assigned to the container
	Memory            int               // max memory in MB
	StorePath         string            // libmachine store path
	OCIBinary         string            // oci tool to use (docker, podman,...)
	ImageDigest       string            // image name with sha to use for the node
	Mounts            []oci.Mount       // mounts
	APIServerPort     int               // Kubernetes api server port inside the container
	PortMappings      []oci.PortMapping // container port mappings
	Envs              map[string]string // key,value of environment variables passed to the node
	KubernetesVersion string            // Kubernetes version to install
	ContainerRuntime  string            // container runtime kic is running
}
