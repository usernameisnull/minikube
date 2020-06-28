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

package image

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/golang/glog"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"k8s.io/minikube/pkg/minikube/constants"
)

var defaultPlatform = v1.Platform{
	Architecture: runtime.GOARCH,
	OS:           "linux",
}

// DigestByDockerLib uses client by docker lib to return image digest
// img.ID in as same as image digest
func DigestByDockerLib(imgClient *client.Client, imgName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	imgClient.NegotiateAPIVersion(ctx)
	img, _, err := imgClient.ImageInspectWithRaw(ctx, imgName)
	if err != nil && !client.IsErrNotFound(err) {
		glog.Infof("couldn't find image digest %s from local daemon: %v ", imgName, err)
		return ""
	}
	return img.ID
}

// DigestByGoLib gets image digest uses go-containerregistry lib
// which is 4s slower thabn local daemon per lookup https://github.com/google/go-containerregistry/issues/627
func DigestByGoLib(imgName string) string {
	ref, err := name.ParseReference(imgName, name.WeakValidation)
	if err != nil {
		glog.Infof("error parsing image name %s ref %v ", imgName, err)
		return ""
	}

	img, err := retrieveImage(ref)
	if err != nil {
		glog.Infof("error retrieve Image %s ref %v ", imgName, err)
		return ""
	}

	cf, err := img.ConfigName()
	if err != nil {
		glog.Infof("error getting Image config name %s %v ", imgName, err)
		return cf.Hex
	}
	return cf.Hex
}

// ExistsImageInDaemon if img exist in local docker daemon
func ExistsImageInDaemon(img string) bool {
	// Check if image exists locally
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}@{{.Digest}}")
	if output, err := cmd.Output(); err == nil {
		if strings.Contains(string(output), img) {
			glog.Infof("Found %s in local docker daemon, skipping pull", img)
			return true
		}
	}
	// Else, pull it
	return false
}

// WriteImageToDaemon write img to the local docker daemon
func WriteImageToDaemon(img string) error {
	glog.Infof("Writing %s to local daemon", img)
	ref, err := name.ParseReference(img)
	if err != nil {
		return errors.Wrap(err, "parsing reference")
	}
	glog.V(3).Infof("Getting image %v", ref)
	i, err := remote.Image(ref)
	if err != nil {
		if strings.Contains(err.Error(), "GitHub Docker Registry needs login") {
			ErrGithubNeedsLogin = errors.New(err.Error())
			return ErrGithubNeedsLogin
		} else if strings.Contains(err.Error(), "UNAUTHORIZED") {
			ErrNeedsLogin = errors.New(err.Error())
			return ErrNeedsLogin
		}

		return errors.Wrap(err, "getting remote image")
	}
	tag, err := name.NewTag(strings.Split(img, "@")[0])
	if err != nil {
		return errors.Wrap(err, "getting tag")
	}
	glog.V(3).Infof("Writing image %v", tag)
	_, err = daemon.Write(tag, i)
	if err != nil {
		return errors.Wrap(err, "writing image")
	}

	//TODO: Make pkg/v1/daemon accept Ref too
	//      Only added it to pkg/v1/tarball
	//
	// https://github.com/google/go-containerregistry/pull/702

	glog.V(3).Infof("Pulling image %v", ref)

	// Pull digest
	cmd := exec.Command("docker", "pull", "--quiet", img)
	if _, err := cmd.Output(); err != nil {
		return errors.Wrap(err, "pulling remote image")
	}

	return nil
}

func retrieveImage(ref name.Reference) (v1.Image, error) {
	glog.Infof("retrieving image: %+v", ref)
	img, err := daemon.Image(ref)
	if err == nil {
		glog.Infof("found %s locally: %+v", ref.Name(), img)
		return img, nil
	}
	// reference does not exist in the local daemon
	if err != nil {
		glog.Infof("daemon lookup for %+v: %v", ref, err)
	}

	platform := defaultPlatform
	img, err = remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithPlatform(platform))
	if err == nil {
		return img, nil
	}

	glog.Warningf("authn lookup for %+v (trying anon): %+v", ref, err)
	img, err = remote.Image(ref)
	return img, err
}

func cleanImageCacheDir() error {
	err := filepath.Walk(constants.ImageCacheDir, func(path string, info os.FileInfo, err error) error {
		// If error is not nil, it's because the path was already deleted and doesn't exist
		// Move on to next path
		if err != nil {
			return nil
		}
		// Check if path is directory
		if !info.IsDir() {
			return nil
		}
		// If directory is empty, delete it
		entries, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err = os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
