#!/bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

export K8SRELEASE=v1.10.8

rm -fr temp
mkdir temp
cd temp

wget https://storage.googleapis.com/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubeadm
ossutil cp kubeadm oss://kubernetes/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubeadm
wget https://storage.googleapis.com/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubelet
ossutil cp kubelet oss://kubernetes/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubelet

wget https://storage.googleapis.com/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubeadm.sha1
ossutil cp kubeadm.sha1 oss://kubernetes/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubeadm.sha1
wget https://storage.googleapis.com/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubelet.sha1
ossutil cp kubelet.sha1 oss://kubernetes/kubernetes-release/release/$K8SRELEASE/bin/linux/amd64/kubelet.sha1

wget https://storage.googleapis.com/kubernetes-release/release/stable-1.txt
ossutil cp -f stable-1.txt oss://kubernetes/kubernetes-release/release/stable-1.txt
cd ..
rm -fr temp

