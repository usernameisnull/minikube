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

package assets

import (
	"runtime"

	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/vmpath"
)

// Addon is a named list of assets, that can be enabled
type Addon struct {
	Assets    []*BinAsset
	enabled   bool
	addonName string
}

// NewAddon creates a new Addon
func NewAddon(assets []*BinAsset, enabled bool, addonName string) *Addon {
	a := &Addon{
		Assets:    assets,
		enabled:   enabled,
		addonName: addonName,
	}
	return a
}

// Name get the addon name
func (a *Addon) Name() string {
	return a.addonName
}

// IsEnabled checks if an Addon is enabled for the given profile
func (a *Addon) IsEnabled(cc *config.ClusterConfig) bool {
	status, ok := cc.Addons[a.Name()]
	if ok {
		return status
	}

	// Return the default unconfigured state of the addon
	return a.enabled
}

// Addons is the list of addons
// TODO: Make dynamically loadable: move this data to a .yaml file within each addon directory
var Addons = map[string]*Addon{
	"dashboard": NewAddon([]*BinAsset{
		// We want to create the kubernetes-dashboard ns first so that every subsequent object can be created
		MustBinAsset("deploy/addons/dashboard/dashboard-ns.yaml", vmpath.GuestAddonsDir, "dashboard-ns.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-clusterrole.yaml", vmpath.GuestAddonsDir, "dashboard-clusterrole.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-clusterrolebinding.yaml", vmpath.GuestAddonsDir, "dashboard-clusterrolebinding.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-configmap.yaml", vmpath.GuestAddonsDir, "dashboard-configmap.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-dp.yaml", vmpath.GuestAddonsDir, "dashboard-dp.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-role.yaml", vmpath.GuestAddonsDir, "dashboard-role.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-rolebinding.yaml", vmpath.GuestAddonsDir, "dashboard-rolebinding.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-sa.yaml", vmpath.GuestAddonsDir, "dashboard-sa.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-secret.yaml", vmpath.GuestAddonsDir, "dashboard-secret.yaml", "0640", false),
		MustBinAsset("deploy/addons/dashboard/dashboard-svc.yaml", vmpath.GuestAddonsDir, "dashboard-svc.yaml", "0640", false),
	}, false, "dashboard"),
	"default-storageclass": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storageclass/storageclass.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"storageclass.yaml",
			"0640",
			false),
	}, true, "default-storageclass"),
	"storage-provisioner": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storage-provisioner/storage-provisioner.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"storage-provisioner.yaml",
			"0640",
			true),
	}, true, "storage-provisioner"),
	"storage-provisioner-gluster": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/storage-gluster-ns.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"storage-gluster-ns.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/glusterfs-daemonset.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"glusterfs-daemonset.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/heketi-deployment.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"heketi-deployment.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/storage-provisioner-gluster/storage-provisioner-glusterfile.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"storage-privisioner-glusterfile.yaml",
			"0640",
			false),
	}, false, "storage-provisioner-gluster"),
	"efk": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/efk/elasticsearch-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"elasticsearch-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/efk/elasticsearch-svc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"elasticsearch-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/fluentd-es-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"fluentd-es-rc.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/efk/fluentd-es-configmap.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"fluentd-es-configmap.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/kibana-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"kibana-rc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/efk/kibana-svc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"kibana-svc.yaml",
			"0640",
			false),
	}, false, "efk"),
	"ingress": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/ingress/ingress-configmap.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"ingress-configmap.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ingress/ingress-rbac.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"ingress-rbac.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ingress/ingress-dp.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"ingress-dp.yaml",
			"0640",
			true),
	}, false, "ingress"),
	"istio-provisioner": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/istio-provisioner/istio-operator.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"istio-operator.yaml",
			"0640",
			true),
	}, false, "istio-provisioner"),
	"istio": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/istio/istio-default-profile.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"istio-default-profile.yaml",
			"0640",
			false),
	}, false, "istio"),
	"metrics-server": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-apiservice.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"metrics-apiservice.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-server-deployment.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"metrics-server-deployment.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/metrics-server/metrics-server-service.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"metrics-server-service.yaml",
			"0640",
			false),
	}, false, "metrics-server"),
	"olm": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/olm/crds.yaml",
			vmpath.GuestAddonsDir,
			"crds.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/olm/olm.yaml",
			vmpath.GuestAddonsDir,
			"olm.yaml",
			"0640",
			false),
	}, false, "olm"),
	"registry": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/registry/registry-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"registry-rc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry/registry-svc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"registry-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry/registry-proxy.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"registry-proxy.yaml",
			"0640",
			false),
	}, false, "registry"),
	"registry-creds": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/registry-creds/registry-creds-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"registry-creds-rc.yaml",
			"0640",
			false),
	}, false, "registry-creds"),
	"registry-aliases": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/registry-aliases/registry-aliases-sa.tmpl",
			vmpath.GuestAddonsDir,
			"registry-aliases-sa.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry-aliases/registry-aliases-sa-crb.tmpl",
			vmpath.GuestAddonsDir,
			"registry-aliases-sa-crb.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry-aliases/registry-aliases-config.tmpl",
			vmpath.GuestAddonsDir,
			"registry-aliases-config.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry-aliases/node-etc-hosts-update.tmpl",
			vmpath.GuestAddonsDir,
			"node-etc-hosts-update.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/registry-aliases/patch-coredns-job.tmpl",
			vmpath.GuestAddonsDir,
			"patch-coredns-job.yaml",
			"0640",
			false),
	}, false, "registry-aliases"),
	"freshpod": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/freshpod/freshpod-rc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"freshpod-rc.yaml",
			"0640",
			true),
	}, false, "freshpod"),
	"nvidia-driver-installer": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gpu/nvidia-driver-installer.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"nvidia-driver-installer.yaml",
			"0640",
			true),
	}, false, "nvidia-driver-installer"),
	"nvidia-gpu-device-plugin": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gpu/nvidia-gpu-device-plugin.yaml",
			vmpath.GuestAddonsDir,
			"nvidia-gpu-device-plugin.yaml",
			"0640",
			false),
	}, false, "nvidia-gpu-device-plugin"),
	"logviewer": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/logviewer/logviewer-dp-and-svc.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"logviewer-dp-and-svc.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/logviewer/logviewer-rbac.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"logviewer-rbac.yaml",
			"0640",
			false),
	}, false, "logviewer"),
	"gvisor": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-pod.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"gvisor-pod.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-runtimeclass.yaml",
			vmpath.GuestAddonsDir,
			"gvisor-runtimeclass.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/gvisor/gvisor-config.toml",
			vmpath.GuestGvisorDir,
			constants.GvisorConfigTomlTargetName,
			"0640",
			true),
	}, false, "gvisor"),
	"helm-tiller": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/helm-tiller/helm-tiller-dp.tmpl",
			vmpath.GuestAddonsDir,
			"helm-tiller-dp.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/helm-tiller/helm-tiller-rbac.tmpl",
			vmpath.GuestAddonsDir,
			"helm-tiller-rbac.yaml",
			"0640",
			true),
		MustBinAsset(
			"deploy/addons/helm-tiller/helm-tiller-svc.tmpl",
			vmpath.GuestAddonsDir,
			"helm-tiller-svc.yaml",
			"0640",
			true),
	}, false, "helm-tiller"),
	"ingress-dns": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/ingress-dns/ingress-dns-pod.yaml",
			vmpath.GuestAddonsDir,
			"ingress-dns-pod.yaml",
			"0640",
			false),
	}, false, "ingress-dns"),
	"metallb": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/metallb/metallb.yaml",
			vmpath.GuestAddonsDir,
			"metallb.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/metallb/metallb-config.yaml.tmpl",
			vmpath.GuestAddonsDir,
			"metallb-config.yaml",
			"0640",
			true),
	}, false, "metallb"),
	"ambassador": NewAddon([]*BinAsset{
		MustBinAsset(
			"deploy/addons/ambassador/ambassador-operator-crds.yaml",
			vmpath.GuestAddonsDir,
			"ambassador-operator-crds.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ambassador/ambassador-operator.yaml",
			vmpath.GuestAddonsDir,
			"ambassador-operator.yaml",
			"0640",
			false),
		MustBinAsset(
			"deploy/addons/ambassador/ambassadorinstallation.yaml",
			vmpath.GuestAddonsDir,
			"ambassadorinstallation.yaml.yaml",
			"0640",
			false),
	}, false, "ambassador"),
}

// GenerateTemplateData generates template data for template assets
func GenerateTemplateData(cfg config.KubernetesConfig) interface{} {

	a := runtime.GOARCH
	// Some legacy docker images still need the -arch suffix
	// for  less common architectures blank suffix for amd64
	ea := ""
	if runtime.GOARCH != "amd64" {
		ea = "-" + runtime.GOARCH
	}
	opts := struct {
		Arch                string
		ExoticArch          string
		ImageRepository     string
		LoadBalancerStartIP string
		LoadBalancerEndIP   string
	}{
		Arch:                a,
		ExoticArch:          ea,
		ImageRepository:     cfg.ImageRepository,
		LoadBalancerStartIP: cfg.LoadBalancerStartIP,
		LoadBalancerEndIP:   cfg.LoadBalancerEndIP,
	}

	return opts
}
