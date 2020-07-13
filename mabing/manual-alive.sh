#!/bin/bash
# 在minikube集群还没delete的时候执行
destDir='/root/.minikube/cache/linux/v1.16.2/crts/'
mkdir -p ${destDir}
cp /root/.minikube/profiles/minikube/apiserver.crt  ${destDir}
cp /root/.minikube/profiles/minikube/apiserver.key  ${destDir} 
cp /root/.minikube/profiles/minikube/proxy-client.crt  ${destDir}
cp /root/.minikube/profiles/minikube/proxy-client.key  ${destDir}
cp /root/.minikube/ca.crt ${destDir}
cp /root/.minikube/ca.key ${destDir}
cp /root/.minikube/proxy-client-ca.crt ${destDir}
cp /root/.minikube/proxy-client-ca.key ${destDir}
cp /root/.minikube/ca.crt ${destDir}
