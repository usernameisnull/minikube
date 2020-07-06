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

package node

import (
	"os"
	"runtime"
	"strings"

	"k8s.io/minikube/mabing"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	cmdcfg "k8s.io/minikube/cmd/minikube/cmd/config"
	"k8s.io/minikube/pkg/drivers/kic"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/download"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/image"
	"k8s.io/minikube/pkg/minikube/localpath"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/out"
)

const (
	cacheImages         = "cache-images"
	cacheImageConfigKey = "cache"
)

// BeginCacheKubernetesImages caches images required for Kubernetes version in the background
func beginCacheKubernetesImages(g *errgroup.Group, imageRepository string, k8sVersion string, cRuntime string) {
	mabing.Logln(mabing.GenerateLongSignStart("beginCacheKubernetesImages"))
	mabing.Logf("g = %+v, imageRepository = %+v, k8sVersion = %+v, cRuntime = %+v", g, imageRepository, k8sVersion, cRuntime)
	// TODO: remove imageRepository check once #7695 is fixed
	if imageRepository == "" && download.PreloadExists(k8sVersion, cRuntime) {
		glog.Info("Caching tarball of preloaded images")
		err := download.Preload(k8sVersion, cRuntime)
		if err == nil {
			glog.Infof("Finished verifying existence of preloaded tar for  %s on %s", k8sVersion, cRuntime)
			return // don't cache individual images if preload is successful.
		}
		glog.Warningf("Error downloading preloaded artifacts will continue without preload: %v", err)
	}

	if !viper.GetBool(cacheImages) {
		return
	}

	g.Go(func() error {
		return machine.CacheImagesForBootstrapper(imageRepository, k8sVersion, viper.GetString(cmdcfg.Bootstrapper))
	})
	mabing.Logln(mabing.GenerateLongSignEnd("beginCacheKubernetesImages"))
}

// HandleDownloadOnly caches appropariate binaries and images
func handleDownloadOnly(cacheGroup, kicGroup *errgroup.Group, k8sVersion string) {
	// If --download-only, complete the remaining downloads and exit.
	if !viper.GetBool("download-only") {
		return
	}
	if err := doCacheBinaries(k8sVersion); err != nil {
		exit.WithError("Failed to cache binaries", err)
	}
	if _, err := CacheKubectlBinary(k8sVersion); err != nil {
		exit.WithError("Failed to cache kubectl", err)
	}
	waitCacheRequiredImages(cacheGroup)
	waitDownloadKicArtifacts(kicGroup)
	if err := saveImagesToTarFromConfig(); err != nil {
		exit.WithError("Failed to cache images to tar", err)
	}
	out.T(out.Check, "Download complete!")
	os.Exit(0)
}

// CacheKubectlBinary caches the kubectl binary
func CacheKubectlBinary(k8sVerison string) (string, error) {
	binary := "kubectl"
	if runtime.GOOS == "windows" {
		binary = "kubectl.exe"
	}

	return download.Binary(binary, k8sVerison, runtime.GOOS, runtime.GOARCH)
}

// doCacheBinaries caches Kubernetes binaries in the foreground
func doCacheBinaries(k8sVersion string) error {
	return machine.CacheBinariesForBootstrapper(k8sVersion, viper.GetString(cmdcfg.Bootstrapper))
}

// BeginDownloadKicArtifacts downloads the kic image + preload tarball, returns true if preload is available
func beginDownloadKicArtifacts(g *errgroup.Group, cc *config.ClusterConfig) {
	glog.Infof("Beginning downloading kic artifacts for %s with %s", cc.Driver, cc.KubernetesConfig.ContainerRuntime)
	if cc.Driver == "docker" {
		if !image.ExistsImageInDaemon(cc.KicBaseImage) {
			out.T(out.Pulling, "Pulling base image ...")
			g.Go(func() error {
				// TODO #8004 : make base-image respect --image-repository
				glog.Infof("Downloading %s to local daemon", cc.KicBaseImage)
				err := image.WriteImageToDaemon(cc.KicBaseImage)
				if err != nil {
					glog.Infof("failed to download base-image %q will try to download the fallback base-image %q instead.", cc.KicBaseImage, kic.BaseImageFallBack1)
					cc.KicBaseImage = kic.BaseImageFallBack1
					if err := image.WriteImageToDaemon(kic.BaseImageFallBack1); err != nil {
						cc.KicBaseImage = kic.BaseImageFallBack2
						glog.Infof("failed to docker hub base-image %q will try to download the github packages base-image %q instead.", cc.KicBaseImage, kic.BaseImageFallBack2)
						return image.WriteImageToDaemon(kic.BaseImageFallBack2)
					}
				}
				return nil
			})
		}
	} else {
		// TODO: driver == "podman"
		glog.Info("Driver isn't docker, skipping base-image download")
	}
}

// WaitDownloadKicArtifacts blocks until the required artifacts for KIC are downloaded.
func waitDownloadKicArtifacts(g *errgroup.Group) {
	if err := g.Wait(); err != nil {
		if err != nil {
			if errors.Is(err, image.ErrGithubNeedsLogin) {
				glog.Warningf("Error downloading kic artifacts: %v", err)
				out.ErrT(out.Connectivity, "Unfortunately, could not download the base image {{.image_name}} ", out.V{"image_name": strings.Split(kic.BaseImage, "@")[0]})
				out.WarningT("In order to use the fall back image, you need to log in to the github packages registry")
				out.T(out.Documentation, `Please visit the following link for documentation around this: 
	https://help.github.com/en/packages/using-github-packages-with-your-projects-ecosystem/configuring-docker-for-use-with-github-packages#authenticating-to-github-packages
`)
			}
			if errors.Is(err, image.ErrGithubNeedsLogin) || errors.Is(err, image.ErrNeedsLogin) {
				exit.UsageT(`Please either authenticate to the registry or use --base-image flag to use a different registry.`)
			} else {
				glog.Errorln("Error downloading kic artifacts: ", err)
			}

		}

	}
	glog.Info("Successfully downloaded all kic artifacts")
}

// WaitCacheRequiredImages blocks until the required images are all cached.
func waitCacheRequiredImages(g *errgroup.Group) {
	if !viper.GetBool(cacheImages) {
		return
	}
	if err := g.Wait(); err != nil {
		glog.Errorln("Error caching images: ", err)
	}
}

// saveImagesToTarFromConfig saves images to tar in cache which specified in config file.
// currently only used by download-only option
func saveImagesToTarFromConfig() error {
	images, err := imagesInConfigFile()
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return nil
	}
	return image.SaveToDir(images, constants.ImageCacheDir)
}

// CacheAndLoadImagesInConfig loads the images currently in the config file
// called by 'start' and 'cache reload' commands.
func CacheAndLoadImagesInConfig() error {
	images, err := imagesInConfigFile()
	if err != nil {
		return errors.Wrap(err, "images")
	}
	if len(images) == 0 {
		return nil
	}
	return machine.CacheAndLoadImages(images)
}

func imagesInConfigFile() ([]string, error) {
	configFile, err := config.ReadConfig(localpath.ConfigFile())
	if err != nil {
		return nil, errors.Wrap(err, "read")
	}
	if values, ok := configFile[cacheImageConfigKey]; ok {
		var images []string
		for key := range values.(map[string]interface{}) {
			images = append(images, key)
		}
		return images, nil
	}
	return []string{}, nil
}
