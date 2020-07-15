#!/bin/bash
sourceDir='/root/.minikube/cache/linux/v1.16.2'
mkdir -p /var/lib/minikube/binaries/v1.16.2
cp ${sourceDir}/kubelet /var/lib/minikube/binaries/v1.16.2/kubelet
cp ${sourceDir}/crts/config.yaml /var/lib/kubelet/config.yaml
# 复制证书相关
mkdir -p /var/lib/minikube/certs
mkdir -p /usr/share/ca-certificates
crtsDir="${sourceDir}/crts"
# 下面这些在minikube delete之后就没有了
cp ${crtsDir}/apiserver.crt  /var/lib/minikube/certs/apiserver.crt
cp ${crtsDir}/apiserver.key /var/lib/minikube/certs/apiserver.key
cp ${crtsDir}/proxy-client.crt  /var/lib/minikube/certs/proxy-client.crt
cp ${crtsDir}/proxy-client.key  /var/lib/minikube/certs/proxy-client.key
cp ${crtsDir}/ca.crt /var/lib/minikube/certs/ca.crt
cp ${crtsDir}/ca.key /var/lib/minikube/certs/ca.key
cp ${crtsDir}/proxy-client-ca.crt /var/lib/minikube/certs/proxy-client-ca.crt 
cp ${crtsDir}/proxy-client-ca.key /var/lib/minikube/certs/proxy-client-ca.key
cp ${crtsDir}/ca.crt /usr/share/ca-certificates/minikubeCA.pem
configJsonPath='/root/.minikube/profiles/minikube'
mkdir -p ${configJsonPath}
cp ${sourceDir}/config.json ${configJsonPath}/config.json

# 把machine-config.json复制到默认的目录
machineDir='/root/.minikube/machines/minikube'
mkdir -p ${machineDir}
cp ${sourceDir}/machine-config.json ${machineDir}/config.json

cp /tmp/config /root/.kube/config

cp /tmp/client.crt /root/.minikube/profiles/minikube/client.crt
cp /tmp/client.key /root/.minikube/profiles/minikube/client.key
