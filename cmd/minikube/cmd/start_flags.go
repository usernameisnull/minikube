/*
Copyright 2020 The Kubernetes Authors All rights reserved.

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

package cmd

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/minikube/mabing"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil"
	"k8s.io/minikube/pkg/minikube/bootstrapper/bsutil/kverify"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/download"
	"k8s.io/minikube/pkg/minikube/driver"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/out"
	"k8s.io/minikube/pkg/minikube/proxy"
	pkgutil "k8s.io/minikube/pkg/util"
	"k8s.io/minikube/pkg/version"
)

const (
	isoURL                  = "iso-url"
	memory                  = "memory"
	cpus                    = "cpus"
	humanReadableDiskSize   = "disk-size"
	nfsSharesRoot           = "nfs-shares-root"
	nfsShare                = "nfs-share"
	kubernetesVersion       = "kubernetes-version"
	hostOnlyCIDR            = "host-only-cidr"
	containerRuntime        = "container-runtime"
	criSocket               = "cri-socket"
	networkPlugin           = "network-plugin"
	enableDefaultCNI        = "enable-default-cni"
	hypervVirtualSwitch     = "hyperv-virtual-switch"
	hypervUseExternalSwitch = "hyperv-use-external-switch"
	hypervExternalAdapter   = "hyperv-external-adapter"
	kvmNetwork              = "kvm-network"
	kvmQemuURI              = "kvm-qemu-uri"
	kvmGPU                  = "kvm-gpu"
	kvmHidden               = "kvm-hidden"
	minikubeEnvPrefix       = "MINIKUBE"
	installAddons           = "install-addons"
	defaultDiskSize         = "20000mb"
	keepContext             = "keep-context"
	createMount             = "mount"
	featureGates            = "feature-gates"
	apiServerName           = "apiserver-name"
	apiServerPort           = "apiserver-port"
	dnsDomain               = "dns-domain"
	serviceCIDR             = "service-cluster-ip-range"
	imageRepository         = "image-repository"
	imageMirrorCountry      = "image-mirror-country"
	mountString             = "mount-string"
	disableDriverMounts     = "disable-driver-mounts"
	cacheImages             = "cache-images"
	uuid                    = "uuid"
	vpnkitSock              = "hyperkit-vpnkit-sock"
	vsockPorts              = "hyperkit-vsock-ports"
	embedCerts              = "embed-certs"
	noVTXCheck              = "no-vtx-check"
	downloadOnly            = "download-only"
	dnsProxy                = "dns-proxy"
	hostDNSResolver         = "host-dns-resolver"
	waitComponents          = "wait"
	force                   = "force"
	dryRun                  = "dry-run"
	interactive             = "interactive"
	waitTimeout             = "wait-timeout"
	nativeSSH               = "native-ssh"
	minUsableMem            = 1024 // Kubernetes will not start with less than 1GB
	minRecommendedMem       = 2000 // Warn at no lower than existing configurations
	minimumCPUS             = 2
	minimumDiskSize         = 2000
	autoUpdate              = "auto-update-drivers"
	hostOnlyNicType         = "host-only-nic-type"
	natNicType              = "nat-nic-type"
	nodes                   = "nodes"
	preload                 = "preload"
	deleteOnFailure         = "delete-on-failure"
	forceSystemd            = "force-systemd"
	kicBaseImage            = "base-image"
)

// initMinikubeFlags includes commandline flags for minikube.
func initMinikubeFlags() {
	viper.SetEnvPrefix(minikubeEnvPrefix)
	// Replaces '-' in flags with '_' in env variables
	// e.g. iso-url => $ENVPREFIX_ISO_URL
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	startCmd.Flags().Bool(force, false, "Force minikube to perform possibly dangerous operations")
	startCmd.Flags().Bool(interactive, true, "Allow user prompts for more information")
	startCmd.Flags().Bool(dryRun, false, "dry-run mode. Validates configuration, but does not mutate system state")

	startCmd.Flags().Int(cpus, 2, "Number of CPUs allocated to Kubernetes.")
	startCmd.Flags().String(memory, "", "Amount of RAM to allocate to Kubernetes (format: <number>[<unit>], where unit = b, k, m or g).")
	startCmd.Flags().String(humanReadableDiskSize, defaultDiskSize, "Disk size allocated to the minikube VM (format: <number>[<unit>], where unit = b, k, m or g).")
	startCmd.Flags().Bool(downloadOnly, false, "If true, only download and cache files for later use - don't install or start anything.")
	startCmd.Flags().Bool(cacheImages, true, "If true, cache docker images for the current bootstrapper and load them into the machine. Always false with --driver=none.")
	startCmd.Flags().StringSlice(isoURL, download.DefaultISOURLs(), "Locations to fetch the minikube ISO from.")
	startCmd.Flags().String(kicBaseImage, kic.BaseImage, "The base image to use for docker/podman drivers. Intended for local development.")
	startCmd.Flags().Bool(keepContext, false, "This will keep the existing kubectl context and will create a minikube context.")
	startCmd.Flags().Bool(embedCerts, false, "if true, will embed the certs in kubeconfig.")
	startCmd.Flags().String(containerRuntime, "docker", "The container runtime to be used (docker, crio, containerd).")
	startCmd.Flags().Bool(createMount, false, "This will start the mount daemon and automatically mount files into minikube.")
	startCmd.Flags().String(mountString, constants.DefaultMountDir+":/minikube-host", "The argument to pass the minikube mount command on start.")
	startCmd.Flags().StringArrayVar(&config.AddonList, "addons", nil, "Enable addons. see `minikube addons list` for a list of valid addon names.")
	startCmd.Flags().String(criSocket, "", "The cri socket path to be used.")
	startCmd.Flags().String(networkPlugin, "", "The name of the network plugin.")
	startCmd.Flags().Bool(enableDefaultCNI, false, "Enable the default CNI plugin (/etc/cni/net.d/k8s.conf). Used in conjunction with \"--network-plugin=cni\".")
	startCmd.Flags().StringSlice(waitComponents, kverify.DefaultWaitList, fmt.Sprintf("comma separated list of Kubernetes components to verify and wait for after starting a cluster. defaults to %q, available options: %q . other acceptable values are 'all' or 'none', 'true' and 'false'", strings.Join(kverify.DefaultWaitList, ","), strings.Join(kverify.AllComponentsList, ",")))
	startCmd.Flags().Duration(waitTimeout, 6*time.Minute, "max time to wait per Kubernetes core services to be healthy.")
	startCmd.Flags().Bool(nativeSSH, true, "Use native Golang SSH client (default true). Set to 'false' to use the command line 'ssh' command when accessing the docker machine. Useful for the machine drivers when they will not start with 'Waiting for SSH'.")
	startCmd.Flags().Bool(autoUpdate, true, "If set, automatically updates drivers to the latest version. Defaults to true.")
	startCmd.Flags().Bool(installAddons, true, "If set, install addons. Defaults to true.")
	startCmd.Flags().IntP(nodes, "n", 1, "The number of nodes to spin up. Defaults to 1.")
	startCmd.Flags().Bool(preload, true, "If set, download tarball of preloaded images if available to improve start time. Defaults to true.")
	startCmd.Flags().Bool(deleteOnFailure, false, "If set, delete the current cluster if start fails and try again. Defaults to false.")
	startCmd.Flags().Bool(forceSystemd, false, "If set, force the container runtime to use sytemd as cgroup manager. Currently available for docker and crio. Defaults to false.")
}

// initKubernetesFlags inits the commandline flags for Kubernetes related options
func initKubernetesFlags() {
	startCmd.Flags().String(kubernetesVersion, "", fmt.Sprintf("The Kubernetes version that the minikube VM will use (ex: v1.2.3, 'stable' for %s, 'latest' for %s). Defaults to 'stable'.", constants.DefaultKubernetesVersion, constants.NewestKubernetesVersion))
	startCmd.Flags().Var(&config.ExtraOptions, "extra-config",
		`A set of key=value pairs that describe configuration that may be passed to different components.
		The key should be '.' separated, and the first part before the dot is the component to apply the configuration to.
		Valid components are: kubelet, kubeadm, apiserver, controller-manager, etcd, proxy, scheduler
		Valid kubeadm parameters: `+fmt.Sprintf("%s, %s", strings.Join(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmCmdParam], ", "), strings.Join(bsutil.KubeadmExtraArgsWhitelist[bsutil.KubeadmConfigParam], ",")))
	startCmd.Flags().String(featureGates, "", "A set of key=value pairs that describe feature gates for alpha/experimental features.")
	startCmd.Flags().String(dnsDomain, constants.ClusterDNSDomain, "The cluster dns domain name used in the Kubernetes cluster")
	startCmd.Flags().Int(apiServerPort, constants.APIServerPort, "The apiserver listening port")
	startCmd.Flags().String(apiServerName, constants.APIServerName, "The authoritative apiserver hostname for apiserver certificates and connectivity. This can be used if you want to make the apiserver available from outside the machine")
	startCmd.Flags().StringArrayVar(&apiServerNames, "apiserver-names", nil, "A set of apiserver names which are used in the generated certificate for kubernetes.  This can be used if you want to make the apiserver available from outside the machine")
	startCmd.Flags().IPSliceVar(&apiServerIPs, "apiserver-ips", nil, "A set of apiserver IP Addresses which are used in the generated certificate for kubernetes.  This can be used if you want to make the apiserver available from outside the machine")
}

// initDriverFlags inits the commandline flags for vm drivers
func initDriverFlags() {
	startCmd.Flags().String("driver", "", fmt.Sprintf("Driver is one of: %v (defaults to auto-detect)", driver.DisplaySupportedDrivers()))
	startCmd.Flags().String("vm-driver", "", "DEPRECATED, use `driver` instead.")
	startCmd.Flags().Bool(disableDriverMounts, false, "Disables the filesystem mounts provided by the hypervisors")
	startCmd.Flags().Bool("vm", false, "Filter to use only VM Drivers")

	// kvm2
	startCmd.Flags().String(kvmNetwork, "default", "The KVM network name. (kvm2 driver only)")
	startCmd.Flags().String(kvmQemuURI, "qemu:///system", "The KVM QEMU connection URI. (kvm2 driver only)")
	startCmd.Flags().Bool(kvmGPU, false, "Enable experimental NVIDIA GPU support in minikube")
	startCmd.Flags().Bool(kvmHidden, false, "Hide the hypervisor signature from the guest in minikube (kvm2 driver only)")

	// virtualbox
	startCmd.Flags().String(hostOnlyCIDR, "192.168.99.1/24", "The CIDR to be used for the minikube VM (virtualbox driver only)")
	startCmd.Flags().Bool(dnsProxy, false, "Enable proxy for NAT DNS requests (virtualbox driver only)")
	startCmd.Flags().Bool(hostDNSResolver, true, "Enable host resolver for NAT DNS requests (virtualbox driver only)")
	startCmd.Flags().Bool(noVTXCheck, false, "Disable checking for the availability of hardware virtualization before the vm is started (virtualbox driver only)")
	startCmd.Flags().String(hostOnlyNicType, "virtio", "NIC Type used for host only network. One of Am79C970A, Am79C973, 82540EM, 82543GC, 82545EM, or virtio (virtualbox driver only)")
	startCmd.Flags().String(natNicType, "virtio", "NIC Type used for host only network. One of Am79C970A, Am79C973, 82540EM, 82543GC, 82545EM, or virtio (virtualbox driver only)")

	// hyperkit
	startCmd.Flags().StringSlice(vsockPorts, []string{}, "List of guest VSock ports that should be exposed as sockets on the host (hyperkit driver only)")
	startCmd.Flags().String(uuid, "", "Provide VM UUID to restore MAC address (hyperkit driver only)")
	startCmd.Flags().String(vpnkitSock, "", "Location of the VPNKit socket used for networking. If empty, disables Hyperkit VPNKitSock, if 'auto' uses Docker for Mac VPNKit connection, otherwise uses the specified VSock (hyperkit driver only)")
	startCmd.Flags().StringSlice(nfsShare, []string{}, "Local folders to share with Guest via NFS mounts (hyperkit driver only)")
	startCmd.Flags().String(nfsSharesRoot, "/nfsshares", "Where to root the NFS Shares, defaults to /nfsshares (hyperkit driver only)")

	// hyperv
	startCmd.Flags().String(hypervVirtualSwitch, "", "The hyperv virtual switch name. Defaults to first found. (hyperv driver only)")
	startCmd.Flags().Bool(hypervUseExternalSwitch, false, "Whether to use external switch over Default Switch if virtual switch not explicitly specified. (hyperv driver only)")
	startCmd.Flags().String(hypervExternalAdapter, "", "External Adapter on which external switch will be created if no external switch is found. (hyperv driver only)")
}

// initNetworkingFlags inits the commandline flags for connectivity related flags for start
func initNetworkingFlags() {
	startCmd.Flags().StringSliceVar(&insecureRegistry, "insecure-registry", nil, "Insecure Docker registries to pass to the Docker daemon.  The default service CIDR range will automatically be added.")
	startCmd.Flags().StringSliceVar(&registryMirror, "registry-mirror", nil, "Registry mirrors to pass to the Docker daemon")
	startCmd.Flags().String(imageRepository, "", "Alternative image repository to pull docker images from. This can be used when you have limited access to gcr.io. Set it to \"auto\" to let minikube decide one for you. For Chinese mainland users, you may use local gcr.io mirrors such as registry.cn-hangzhou.aliyuncs.com/google_containers")
	startCmd.Flags().String(imageMirrorCountry, "cn", "Country code of the image mirror to be used. Leave empty to use the global one. For Chinese mainland users, set it to cn.")
	startCmd.Flags().String(serviceCIDR, constants.DefaultServiceCIDR, "The CIDR to be used for service cluster IPs.")
	startCmd.Flags().StringArrayVar(&config.DockerEnv, "docker-env", nil, "Environment variables to pass to the Docker daemon. (format: key=value)")
	startCmd.Flags().StringArrayVar(&config.DockerOpt, "docker-opt", nil, "Specify arbitrary flags to pass to the Docker daemon. (format: key=value)")
}

// ClusterFlagValue returns the current cluster name based on flags
func ClusterFlagValue() string {
	return viper.GetString(config.ProfileName) // mabing: cmd/minikube/cmd/root.go里设置了默认值为: minikube
}

// generateClusterConfig generate a config.ClusterConfig based on flags or existing cluster config
func generateClusterConfig(cmd *cobra.Command, existing *config.ClusterConfig, k8sVersion string, drvName string) (config.ClusterConfig, config.Node, error) {
	mabing.Logln(mabing.GenerateLongSignStart("generateClusterConfig()"))
	mabing.Logf("mabing, generateClusterConfig(), cmd = %+v, existing = %+v, k8sVersion = %+v, drvName = %+v", cmd, existing, k8sVersion, drvName)
	var cc config.ClusterConfig
	if existing != nil { // create profile config first time
		cc = updateExistingConfigFromFlags(cmd, existing)
	} else {
		glog.Info("no existing cluster config was found, will generate one from the flags ")
		sysLimit, containerLimit, err := memoryLimits(drvName)
		if err != nil {
			glog.Warningf("Unable to query memory limits: %v", err)
		}

		mem := suggestMemoryAllocation(sysLimit, containerLimit, viper.GetInt(nodes))
		if cmd.Flags().Changed(memory) {
			mem, err = pkgutil.CalculateSizeInMB(viper.GetString(memory))
			if err != nil {
				exit.WithCodeT(exit.Config, "Generate unable to parse memory '{{.memory}}': {{.error}}", out.V{"memory": viper.GetString(memory), "error": err})
			}

		} else {
			glog.Infof("Using suggested %dMB memory alloc based on sys=%dMB, container=%dMB", mem, sysLimit, containerLimit)
		}

		diskSize, err := pkgutil.CalculateSizeInMB(viper.GetString(humanReadableDiskSize))
		if err != nil {
			exit.WithCodeT(exit.Config, "Generate unable to parse disk size '{{.diskSize}}': {{.error}}", out.V{"diskSize": viper.GetString(humanReadableDiskSize), "error": err})
		}

		r, err := cruntime.New(cruntime.Config{Type: viper.GetString(containerRuntime)})
		if err != nil {
			return cc, config.Node{}, errors.Wrap(err, "new runtime manager")
		}

		// Pick good default values for --network-plugin and --enable-default-cni based on runtime.
		selectedEnableDefaultCNI := viper.GetBool(enableDefaultCNI)
		selectedNetworkPlugin := viper.GetString(networkPlugin)
		if r.DefaultCNI() && !cmd.Flags().Changed(networkPlugin) {
			selectedNetworkPlugin = "cni"
			if !cmd.Flags().Changed(enableDefaultCNI) {
				selectedEnableDefaultCNI = true
			}
		}

		repository := viper.GetString(imageRepository)
		mirrorCountry := strings.ToLower(viper.GetString(imageMirrorCountry))
		if strings.ToLower(repository) == "auto" || (mirrorCountry != "" && repository == "") {
			found, autoSelectedRepository, err := selectImageRepository(mirrorCountry, semver.MustParse(strings.TrimPrefix(k8sVersion, version.VersionPrefix)))
			if err != nil {
				exit.WithError("Failed to check main repository and mirrors for images", err)
			}

			if !found {
				if autoSelectedRepository == "" {
					exit.WithCodeT(exit.Failure, "None of the known repositories is accessible. Consider specifying an alternative image repository with --image-repository flag")
				} else {
					out.WarningT("None of the known repositories in your location are accessible. Using {{.image_repository_name}} as fallback.", out.V{"image_repository_name": autoSelectedRepository})
				}
			}

			repository = autoSelectedRepository
		}

		if cmd.Flags().Changed(imageRepository) || cmd.Flags().Changed(imageMirrorCountry) {
			out.T(out.SuccessType, "Using image repository {{.name}}", out.V{"name": repository})
		}

		cc = config.ClusterConfig{
			Name:                    ClusterFlagValue(),
			KeepContext:             viper.GetBool(keepContext),
			EmbedCerts:              viper.GetBool(embedCerts),
			MinikubeISO:             viper.GetString(isoURL),
			KicBaseImage:            viper.GetString(kicBaseImage),
			Memory:                  mem,
			CPUs:                    viper.GetInt(cpus),
			DiskSize:                diskSize,
			Driver:                  drvName,
			HyperkitVpnKitSock:      viper.GetString(vpnkitSock),
			HyperkitVSockPorts:      viper.GetStringSlice(vsockPorts),
			NFSShare:                viper.GetStringSlice(nfsShare),
			NFSSharesRoot:           viper.GetString(nfsSharesRoot),
			DockerEnv:               config.DockerEnv,
			DockerOpt:               config.DockerOpt,
			InsecureRegistry:        insecureRegistry,
			RegistryMirror:          registryMirror,
			HostOnlyCIDR:            viper.GetString(hostOnlyCIDR),
			HypervVirtualSwitch:     viper.GetString(hypervVirtualSwitch),
			HypervUseExternalSwitch: viper.GetBool(hypervUseExternalSwitch),
			HypervExternalAdapter:   viper.GetString(hypervExternalAdapter),
			KVMNetwork:              viper.GetString(kvmNetwork),
			KVMQemuURI:              viper.GetString(kvmQemuURI),
			KVMGPU:                  viper.GetBool(kvmGPU),
			KVMHidden:               viper.GetBool(kvmHidden),
			DisableDriverMounts:     viper.GetBool(disableDriverMounts),
			UUID:                    viper.GetString(uuid),
			NoVTXCheck:              viper.GetBool(noVTXCheck),
			DNSProxy:                viper.GetBool(dnsProxy),
			HostDNSResolver:         viper.GetBool(hostDNSResolver),
			HostOnlyNicType:         viper.GetString(hostOnlyNicType),
			NatNicType:              viper.GetString(natNicType),
			KubernetesConfig: config.KubernetesConfig{
				KubernetesVersion:      k8sVersion,
				ClusterName:            ClusterFlagValue(),
				APIServerName:          viper.GetString(apiServerName),
				APIServerNames:         apiServerNames,
				APIServerIPs:           apiServerIPs,
				DNSDomain:              viper.GetString(dnsDomain),
				FeatureGates:           viper.GetString(featureGates),
				ContainerRuntime:       viper.GetString(containerRuntime),
				CRISocket:              viper.GetString(criSocket),
				NetworkPlugin:          selectedNetworkPlugin,
				ServiceCIDR:            viper.GetString(serviceCIDR),
				ImageRepository:        repository,
				ExtraOptions:           config.ExtraOptions,
				ShouldLoadCachedImages: viper.GetBool(cacheImages),
				EnableDefaultCNI:       selectedEnableDefaultCNI,
				NodePort:               viper.GetInt(apiServerPort),
			},
		}
		cc.VerifyComponents = interpretWaitFlag(*cmd)
	}

	r, err := cruntime.New(cruntime.Config{Type: cc.KubernetesConfig.ContainerRuntime}) //mabing: cc.KubernetesConfig.ContainerRuntime=docker
	mabing.Logln("mabing, generateClusterConfig(), r = ", fmt.Sprintf("%+v", r))
	if err != nil {
		return cc, config.Node{}, errors.Wrap(err, "new runtime manager")
	}

	// Feed Docker our host proxy environment by default, so that it can pull images
	// doing this for both new config and existing, in case proxy changed since previous start
	if _, ok := r.(*cruntime.Docker); ok {
		mabing.Logln("mabing, generateClusterConfig(), 开始设置Docker环境变量")
		proxy.SetDockerEnv()
	}

	var kubeNodeName string
	if driver.BareMetal(cc.Driver) {
		kubeNodeName = "m01"
	}
	mabing.Logln("mabing, generateClusterConfig(), kubeNodeName = ", kubeNodeName)
	mabing.Logln(mabing.GenerateLongSignEnd("generateClusterConfig()"))
	return createNode(cc, kubeNodeName, existing)
}

// updateExistingConfigFromFlags will update the existing config from the flags - used on a second start
// skipping updating existing docker env , docker opt, InsecureRegistry, registryMirror, extra-config, apiserver-ips
func updateExistingConfigFromFlags(cmd *cobra.Command, existing *config.ClusterConfig) config.ClusterConfig { //nolint to suppress cyclomatic complexity 45 of func `updateExistingConfigFromFlags` is high (> 30)
	validateFlags(cmd, existing.Driver)

	cc := *existing
	if cmd.Flags().Changed(containerRuntime) {
		cc.KubernetesConfig.ContainerRuntime = viper.GetString(containerRuntime)
	}

	if cmd.Flags().Changed(keepContext) {
		cc.KeepContext = viper.GetBool(keepContext)
	}

	if cmd.Flags().Changed(embedCerts) {
		cc.EmbedCerts = viper.GetBool(embedCerts)
	}

	if cmd.Flags().Changed(isoURL) {
		cc.MinikubeISO = viper.GetString(isoURL)
	}

	if cmd.Flags().Changed(memory) {
		memInMB, err := pkgutil.CalculateSizeInMB(viper.GetString(memory))
		if err != nil {
			glog.Warningf("error calculate memory size in mb : %v", err)
		}
		if memInMB != existing.Memory {
			out.WarningT("You cannot change the memory size for an exiting minikube cluster. Please first delete the cluster.")
		}

	}

	if cmd.Flags().Changed(cpus) {
		if viper.GetInt(cpus) != existing.CPUs {
			out.WarningT("You cannot change the CPUs for an exiting minikube cluster. Please first delete the cluster.")
		}
	}

	if cmd.Flags().Changed(humanReadableDiskSize) {
		memInMB, err := pkgutil.CalculateSizeInMB(viper.GetString(humanReadableDiskSize))
		if err != nil {
			glog.Warningf("error calculate disk size in mb : %v", err)
		}

		if memInMB != existing.DiskSize {
			out.WarningT("You cannot change the Disk size for an exiting minikube cluster. Please first delete the cluster.")
		}
	}

	if cmd.Flags().Changed(vpnkitSock) {
		cc.HyperkitVpnKitSock = viper.GetString(vpnkitSock)
	}

	if cmd.Flags().Changed(vsockPorts) {
		cc.HyperkitVSockPorts = viper.GetStringSlice(vsockPorts)
	}

	if cmd.Flags().Changed(nfsShare) {
		cc.NFSShare = viper.GetStringSlice(nfsShare)
	}

	if cmd.Flags().Changed(nfsSharesRoot) {
		cc.NFSSharesRoot = viper.GetString(nfsSharesRoot)
	}

	if cmd.Flags().Changed(hostOnlyCIDR) {
		cc.HostOnlyCIDR = viper.GetString(hostOnlyCIDR)
	}

	if cmd.Flags().Changed(hypervVirtualSwitch) {
		cc.HypervVirtualSwitch = viper.GetString(hypervVirtualSwitch)
	}

	if cmd.Flags().Changed(hypervUseExternalSwitch) {
		cc.HypervUseExternalSwitch = viper.GetBool(hypervUseExternalSwitch)
	}

	if cmd.Flags().Changed(hypervExternalAdapter) {
		cc.HypervExternalAdapter = viper.GetString(hypervExternalAdapter)
	}

	if cmd.Flags().Changed(kvmNetwork) {
		cc.KVMNetwork = viper.GetString(kvmNetwork)
	}

	if cmd.Flags().Changed(kvmQemuURI) {
		cc.KVMQemuURI = viper.GetString(kvmQemuURI)
	}

	if cmd.Flags().Changed(kvmGPU) {
		cc.KVMGPU = viper.GetBool(kvmGPU)
	}

	if cmd.Flags().Changed(kvmHidden) {
		cc.KVMHidden = viper.GetBool(kvmHidden)
	}

	if cmd.Flags().Changed(disableDriverMounts) {
		cc.DisableDriverMounts = viper.GetBool(disableDriverMounts)
	}

	if cmd.Flags().Changed(uuid) {
		cc.UUID = viper.GetString(uuid)
	}

	if cmd.Flags().Changed(noVTXCheck) {
		cc.NoVTXCheck = viper.GetBool(noVTXCheck)
	}

	if cmd.Flags().Changed(dnsProxy) {
		cc.DNSProxy = viper.GetBool(dnsProxy)
	}

	if cmd.Flags().Changed(hostDNSResolver) {
		cc.HostDNSResolver = viper.GetBool(hostDNSResolver)
	}

	if cmd.Flags().Changed(hostOnlyNicType) {
		cc.HostOnlyNicType = viper.GetString(hostOnlyNicType)
	}

	if cmd.Flags().Changed(natNicType) {
		cc.NatNicType = viper.GetString(natNicType)
	}

	if cmd.Flags().Changed(kubernetesVersion) {
		cc.KubernetesConfig.KubernetesVersion = getKubernetesVersion(existing)
	}

	if cmd.Flags().Changed(apiServerName) {
		cc.KubernetesConfig.APIServerName = viper.GetString(apiServerName)
	}

	if cmd.Flags().Changed("apiserver-names") {
		cc.KubernetesConfig.APIServerNames = viper.GetStringSlice("apiserver-names")
	}

	if cmd.Flags().Changed(apiServerPort) {
		cc.KubernetesConfig.NodePort = viper.GetInt(apiServerPort)
	}

	// pre minikube 1.9.2 cc.KubernetesConfig.NodePort was not populated.
	// in minikube config there were two fields for api server port.
	// one in cc.KubernetesConfig.NodePort and one in cc.Nodes.Port
	// this makes sure api server port not be set as 0!
	if existing.KubernetesConfig.NodePort == 0 {
		cc.KubernetesConfig.NodePort = viper.GetInt(apiServerPort)
	}

	if cmd.Flags().Changed(dnsDomain) {
		cc.KubernetesConfig.DNSDomain = viper.GetString(dnsDomain)
	}

	if cmd.Flags().Changed(featureGates) {
		cc.KubernetesConfig.FeatureGates = viper.GetString(featureGates)
	}

	if cmd.Flags().Changed(containerRuntime) {
		cc.KubernetesConfig.ContainerRuntime = viper.GetString(containerRuntime)
	}

	if cmd.Flags().Changed(criSocket) {
		cc.KubernetesConfig.CRISocket = viper.GetString(criSocket)
	}

	if cmd.Flags().Changed(criSocket) {
		cc.KubernetesConfig.NetworkPlugin = viper.GetString(criSocket)
	}

	if cmd.Flags().Changed(networkPlugin) {
		cc.KubernetesConfig.NetworkPlugin = viper.GetString(networkPlugin)
	}

	if cmd.Flags().Changed(serviceCIDR) {
		cc.KubernetesConfig.ServiceCIDR = viper.GetString(serviceCIDR)
	}

	if cmd.Flags().Changed(cacheImages) {
		cc.KubernetesConfig.ShouldLoadCachedImages = viper.GetBool(cacheImages)
	}

	if cmd.Flags().Changed(imageRepository) {
		cc.KubernetesConfig.ImageRepository = viper.GetString(imageRepository)
	}

	if cmd.Flags().Changed(enableDefaultCNI) {
		cc.KubernetesConfig.EnableDefaultCNI = viper.GetBool(enableDefaultCNI)
	}

	if cmd.Flags().Changed(waitComponents) {
		cc.VerifyComponents = interpretWaitFlag(*cmd)
	}

	if cmd.Flags().Changed(kicBaseImage) {
		cc.KicBaseImage = viper.GetString(kicBaseImage)
	}

	return cc
}

// interpretWaitFlag interprets the wait flag and respects the legacy minikube users
// returns map of components to wait for
func interpretWaitFlag(cmd cobra.Command) map[string]bool {
	if !cmd.Flags().Changed(waitComponents) {
		glog.Infof("Wait components to verify : %+v", kverify.DefaultComponents)
		return kverify.DefaultComponents
	}

	waitFlags, err := cmd.Flags().GetStringSlice(waitComponents)
	if err != nil {
		glog.Warningf("Failed to read --wait from flags: %v.\n Moving on will use the default wait components: %+v", err, kverify.DefaultComponents)
		return kverify.DefaultComponents
	}

	if len(waitFlags) == 1 {
		// respecting legacy flag before minikube 1.9.0, wait flag was boolean
		if waitFlags[0] == "false" || waitFlags[0] == "none" {
			glog.Infof("Waiting for no components: %+v", kverify.NoComponents)
			return kverify.NoComponents
		}
		// respecting legacy flag before minikube 1.9.0, wait flag was boolean
		if waitFlags[0] == "true" || waitFlags[0] == "all" {
			glog.Infof("Waiting for all components: %+v", kverify.AllComponents)
			return kverify.AllComponents
		}
	}

	waitComponents := kverify.NoComponents
	for _, wc := range waitFlags {
		seen := false
		for _, valid := range kverify.AllComponentsList {
			if wc == valid {
				waitComponents[wc] = true
				seen = true
				continue
			}
		}
		if !seen {
			glog.Warningf("The value %q is invalid for --wait flag. valid options are %q", wc, strings.Join(kverify.AllComponentsList, ","))
		}
	}
	glog.Infof("Waiting for components: %+v", waitComponents)
	return waitComponents
}
