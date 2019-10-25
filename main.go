/*
k8s标签生成log收集配置文件

1. k8s标签->Pods
2. Pods->ContainerIds
3. ContainerIds->logHostPath
4. logHostPath->配置文件

*/

package main

import (
    "k8s.io/klog"
    "os"
    "time"

    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

const (
    moduleNameTag     = "torrent/module_name"
    logPathTag        = "torrent/log_path"
    inputTemplatePath = "/tpl/filebeat-input-log.tpl"
)

var (
    dockerClientVersion string
)

func init() {
    if os.Getenv("DOCKER_CLIENT_VERSION") == "" {
        dockerClientVersion = "1.38"
    } else {
        dockerClientVersion = os.Getenv("DOCKER_CLIENT_VERSION")
    }
}

func main() {
    klog.InitFlags(nil)

    stopCh := make(chan struct{})

    // creates the in-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
    	klog.Fatalf("Error building kubeconfig: %s", err.Error())
    }

    kubeClient, err := kubernetes.NewForConfig(config)
    if err != nil {
    	klog.Fatalf("Error building kubernets clientset: %s", err.Error())
    }

    kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*30)

    controller := NewController(kubeClient, kubeInformerFactory.Core().V1().Pods())

    kubeInformerFactory.Start(stopCh)

    if err = controller.Run(1, stopCh); err != nil {
        klog.Fatalf("Error running controller: %s", err.Error())
    }

}

