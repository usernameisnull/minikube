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
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	pkgdrivers "k8s.io/minikube/pkg/drivers"
	"k8s.io/minikube/pkg/drivers/kic/oci"
	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/command"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/cruntime"
	"k8s.io/minikube/pkg/minikube/download"
	"k8s.io/minikube/pkg/minikube/sysinit"
	"k8s.io/minikube/pkg/util/retry"
)

// Driver represents a kic driver https://minikube.sigs.k8s.io/docs/reference/drivers/docker
type Driver struct {
	*drivers.BaseDriver
	*pkgdrivers.CommonDriver
	URL        string
	exec       command.Runner
	NodeConfig Config
	OCIBinary  string // docker,podman
}

// NewDriver returns a fully configured Kic driver
func NewDriver(c Config) *Driver {
	d := &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: c.MachineName,
			StorePath:   c.StorePath,
		},
		exec:       command.NewKICRunner(c.MachineName, c.OCIBinary),
		NodeConfig: c,
		OCIBinary:  c.OCIBinary,
	}
	return d
}

// Create a host using the driver's config
func (d *Driver) Create() error {
	params := oci.CreateParams{
		Name:          d.NodeConfig.MachineName,
		Image:         d.NodeConfig.ImageDigest,
		ClusterLabel:  oci.ProfileLabelKey + "=" + d.MachineName,
		NodeLabel:     oci.NodeLabelKey + "=" + d.NodeConfig.MachineName,
		CPUs:          strconv.Itoa(d.NodeConfig.CPU),
		Memory:        strconv.Itoa(d.NodeConfig.Memory) + "mb",
		Envs:          d.NodeConfig.Envs,
		ExtraArgs:     []string{"--expose", fmt.Sprintf("%d", d.NodeConfig.APIServerPort)},
		OCIBinary:     d.NodeConfig.OCIBinary,
		APIServerPort: d.NodeConfig.APIServerPort,
	}

	// control plane specific options
	params.PortMappings = append(params.PortMappings, oci.PortMapping{
		ListenAddress: oci.DefaultBindIPV4,
		ContainerPort: int32(params.APIServerPort),
	},
		oci.PortMapping{
			ListenAddress: oci.DefaultBindIPV4,
			ContainerPort: constants.SSHPort,
		},
		oci.PortMapping{
			ListenAddress: oci.DefaultBindIPV4,
			ContainerPort: constants.DockerDaemonPort,
		},
		oci.PortMapping{
			ListenAddress: oci.DefaultBindIPV4,
			ContainerPort: constants.RegistryAddonPort,
		},
	)

	exists, err := oci.ContainerExists(d.OCIBinary, params.Name, true)
	if err != nil {
		glog.Warningf("failed to check if container already exists: %v", err)
	}
	if exists {
		// if container was created by minikube it is safe to delete and recreate it.
		if oci.IsCreatedByMinikube(d.OCIBinary, params.Name) {
			glog.Info("Found already existing abandoned minikube container, will try to delete.")
			if err := oci.DeleteContainer(d.OCIBinary, params.Name); err != nil {
				glog.Errorf("Failed to delete a conflicting minikube container %s. You might need to restart your %s daemon and delete it manually and try again: %v", params.Name, params.OCIBinary, err)
			}
		} else {
			// The conflicting container name was not created by minikube
			// user has a container that conflicts with minikube profile name, will not delete users container.
			return errors.Wrapf(err, "user has a conflicting container name %q with minikube container. Needs to be deleted by user's consent.", params.Name)
		}
	}

	if err := oci.PrepareContainerNode(params); err != nil {
		return errors.Wrap(err, "setting up container node")
	}

	var waitForPreload sync.WaitGroup
	if d.NodeConfig.OCIBinary == oci.Docker {
		waitForPreload.Add(1)
		go func() {
			defer waitForPreload.Done()
			// If preload doesn't exist, don't bother extracting tarball to volume
			if !download.PreloadExists(d.NodeConfig.KubernetesVersion, d.NodeConfig.ContainerRuntime) {
				return
			}
			t := time.Now()
			glog.Infof("Starting extracting preloaded images to volume ...")
			// Extract preloaded images to container
			if err := oci.ExtractTarballToVolume(d.NodeConfig.OCIBinary, download.TarballPath(d.NodeConfig.KubernetesVersion, d.NodeConfig.ContainerRuntime), params.Name, d.NodeConfig.ImageDigest); err != nil {
				glog.Infof("Unable to extract preloaded tarball to volume: %v", err)
			} else {
				glog.Infof("duration metric: took %f seconds to extract preloaded images to volume", time.Since(t).Seconds())
			}
		}()
	} else {
		// driver == "podman"
		glog.Info("Driver isn't docker, skipping extracting preloaded images")
	}

	if err := oci.CreateContainerNode(params); err != nil {
		return errors.Wrap(err, "create kic node")
	}

	if err := d.prepareSSH(); err != nil {
		return errors.Wrap(err, "prepare kic ssh")
	}

	waitForPreload.Wait()
	return nil
}

// prepareSSH will generate keys and copy to the container so minikube ssh works
func (d *Driver) prepareSSH() error {
	keyPath := d.GetSSHKeyPath()
	glog.Infof("Creating ssh key for kic: %s...", keyPath)
	if err := ssh.GenerateSSHKey(keyPath); err != nil {
		return errors.Wrap(err, "generate ssh key")
	}

	cmder := command.NewKICRunner(d.NodeConfig.MachineName, d.NodeConfig.OCIBinary)
	f, err := assets.NewFileAsset(d.GetSSHKeyPath()+".pub", "/home/docker/.ssh/", "authorized_keys", "0644")
	if err != nil {
		return errors.Wrap(err, "create pubkey assetfile ")
	}
	if err := cmder.Copy(f); err != nil {
		return errors.Wrap(err, "copying pub key")
	}
	if rr, err := cmder.RunCmd(exec.Command("chown", "docker:docker", "/home/docker/.ssh/authorized_keys")); err != nil {
		return errors.Wrapf(err, "apply authorized_keys file ownership, output %s", rr.Output())
	}

	return nil
}

// DriverName returns the name of the driver
func (d *Driver) DriverName() string {
	if d.NodeConfig.OCIBinary == oci.Podman {
		return oci.Podman
	}
	return oci.Docker
}

// GetIP returns an IP or hostname that this host is available at
func (d *Driver) GetIP() (string, error) {
	ip, _, err := oci.ContainerIPs(d.OCIBinary, d.MachineName)
	return ip, err
}

// GetExternalIP returns an IP which is accissble from outside
func (d *Driver) GetExternalIP() (string, error) {
	return oci.DefaultBindIPV4, nil
}

// GetSSHHostname returns hostname for use with ssh
func (d *Driver) GetSSHHostname() (string, error) {
	return oci.DefaultBindIPV4, nil
}

// GetSSHPort returns port for use with ssh
func (d *Driver) GetSSHPort() (int, error) {
	p, err := oci.ForwardedPort(d.OCIBinary, d.MachineName, constants.SSHPort)
	if err != nil {
		return p, errors.Wrap(err, "get ssh host-port")
	}
	return p, nil
}

// GetSSHUsername returns the ssh username
func (d *Driver) GetSSHUsername() string {
	return "docker"
}

// GetSSHKeyPath returns the ssh key path
func (d *Driver) GetSSHKeyPath() string {
	if d.SSHKeyPath == "" {
		d.SSHKeyPath = d.ResolveStorePath("id_rsa")
	}
	return d.SSHKeyPath
}

// GetURL returns a Docker URL inside this host
// e.g. tcp://1.2.3.4:2376
// more info https://github.com/docker/machine/blob/b170508bf44c3405e079e26d5fdffe35a64c6972/libmachine/provision/utils.go#L159_L175
func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("tcp://%s", net.JoinHostPort(ip, "2376"))
	return url, nil
}

// GetState returns the state that the host is in (running, stopped, etc)
func (d *Driver) GetState() (state.State, error) {
	return oci.ContainerStatus(d.OCIBinary, d.MachineName, true)
}

// Kill stops a host forcefully, including any containers that we are managing.
func (d *Driver) Kill() error {
	// on init this doesn't get filled when called from cmd
	d.exec = command.NewKICRunner(d.MachineName, d.OCIBinary)
	if err := sysinit.New(d.exec).ForceStop("kubelet"); err != nil {
		glog.Warningf("couldn't force stop kubelet. will continue with kill anyways: %v", err)
	}

	if err := oci.ShutDown(d.OCIBinary, d.MachineName); err != nil {
		glog.Warningf("couldn't shutdown the container, will continue with kill anyways: %v", err)
	}

	cr := command.NewExecRunner() // using exec runner for interacting with dameon.
	if _, err := cr.RunCmd(oci.PrefixCmd(exec.Command(d.NodeConfig.OCIBinary, "kill", d.MachineName))); err != nil {
		return errors.Wrapf(err, "killing %q", d.MachineName)
	}
	return nil
}

// Remove will delete the Kic Node Container
func (d *Driver) Remove() error {
	if _, err := oci.ContainerID(d.OCIBinary, d.MachineName); err != nil {
		glog.Infof("could not find the container %s to remove it. will try anyways", d.MachineName)
	}

	if err := oci.DeleteContainer(d.NodeConfig.OCIBinary, d.MachineName); err != nil {
		if strings.Contains(err.Error(), "is already in progress") {
			return errors.Wrap(err, "stuck delete")
		}
		if strings.Contains(err.Error(), "No such container:") {
			return nil // nothing was found to delete.
		}

	}

	// check there be no container left after delete
	if id, err := oci.ContainerID(d.OCIBinary, d.MachineName); err == nil && id != "" {
		return fmt.Errorf("expected no container ID be found for %q after delete. but got %q", d.MachineName, id)
	}
	return nil
}

// Restart a host
func (d *Driver) Restart() error {
	s, err := d.GetState()
	if err != nil {
		glog.Warningf("get state during restart: %v", err)
	}
	if s == state.Stopped { // don't stop if already stopped
		return d.Start()
	}
	if err = d.Stop(); err != nil {
		return fmt.Errorf("stop during restart %v", err)
	}
	if err = d.Start(); err != nil {
		return fmt.Errorf("start during restart %v", err)
	}
	return nil
}

// Start an already created kic container
func (d *Driver) Start() error {
	if err := oci.StartContainer(d.NodeConfig.OCIBinary, d.MachineName); err != nil {
		return errors.Wrap(err, "start")
	}
	checkRunning := func() error {
		s, err := oci.ContainerStatus(d.NodeConfig.OCIBinary, d.MachineName)
		if err != nil {
			return err
		}
		if s != state.Running {
			return fmt.Errorf("expected container state be running but got %q", s)
		}
		glog.Infof("container %q state is running.", d.MachineName)
		return nil
	}

	if err := retry.Expo(checkRunning, 500*time.Microsecond, time.Second*30); err != nil {
		return err
	}
	return nil
}

// Stop a host gracefully, including any containers that we are managing.
func (d *Driver) Stop() error {
	// on init this doesn't get filled when called from cmd
	d.exec = command.NewKICRunner(d.MachineName, d.OCIBinary)
	// docker does not send right SIG for systemd to know to stop the systemd.
	// to avoid bind address be taken on an upgrade. more info https://github.com/kubernetes/minikube/issues/7171
	if err := sysinit.New(d.exec).Stop("kubelet"); err != nil {
		glog.Warningf("couldn't stop kubelet. will continue with stop anyways: %v", err)
		if err := sysinit.New(d.exec).ForceStop("kubelet"); err != nil {
			glog.Warningf("couldn't force stop kubelet. will continue with stop anyways: %v", err)
		}
	}

	runtime, err := cruntime.New(cruntime.Config{Type: d.NodeConfig.ContainerRuntime, Runner: d.exec})
	if err != nil { // won't return error because:
		// even though we can't stop the cotainers inside, we still wanna stop the minikube container itself
		glog.Errorf("unable to get container runtime: %v", err)
	} else {
		containers, err := runtime.ListContainers(cruntime.ListOptions{Namespaces: constants.DefaultNamespaces})
		if err != nil {
			glog.Infof("unable list containers : %v", err)
		}
		if len(containers) > 0 {
			if err := runtime.StopContainers(containers); err != nil {
				glog.Infof("unable to stop containers : %v", err)
			}
			if err := runtime.KillContainers(containers); err != nil {
				glog.Errorf("unable to kill containers : %v", err)
			}
		}
		glog.Infof("successfully stopped kubernetes!")

	}

	if err := killAPIServerProc(d.exec); err != nil {
		glog.Warningf("couldn't stop kube-apiserver proc: %v", err)
	}

	cmd := exec.Command(d.NodeConfig.OCIBinary, "stop", d.MachineName)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "stopping %s", d.MachineName)
	}
	return nil
}

// RunSSHCommandFromDriver implements direct ssh control to the driver
func (d *Driver) RunSSHCommandFromDriver() error {
	return fmt.Errorf("driver does not support RunSSHCommandFromDriver commands")
}

// killAPIServerProc will kill an api server proc if it exists
// to ensure this never happens https://github.com/kubernetes/minikube/issues/7521
func killAPIServerProc(runner command.Runner) error {
	// first check if it exists
	rr, err := runner.RunCmd(exec.Command("pgrep", "kube-apiserver"))
	if err == nil { // this means we might have a running kube-apiserver
		pid, err := strconv.Atoi(rr.Stdout.String())
		if err == nil { // this means we have a valid pid
			glog.Warningf("Found a kube-apiserver running with pid %d, will try to kill the proc", pid)
			if _, err = runner.RunCmd(exec.Command("pkill", "-9", string(pid))); err != nil {
				return errors.Wrap(err, "kill")
			}
		}
	}
	return nil
}
