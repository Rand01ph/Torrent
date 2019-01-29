/*
k8s标签生成log收集配置文件

1. k8s标签->Pods
2. Pods->ContainerIds
3. ContainerIds->logHostPath
4. logHostPath->配置文件

*/

package main

import (
	"flag"
	"fmt"
	"github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"

	"golang.org/x/net/context"
)

func main() {

	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ctx := context.Background()

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Pods().Informer()
	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			mObj := obj.(*corev1.Pod)
			fmt.Printf("the pod %v is ready ???\n", mObj.Status.Phase)
			if mObj.Status.Phase == corev1.PodRunning {
				return true
			}
			return false
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				mObj := obj.(*corev1.Pod)
				containerId := mObj.Status.ContainerStatuses[0].ContainerID[9:]
				fmt.Printf("New Pod %s Added to Store \t container id is %s\n", mObj.Name, containerId)
				go getHostLogDir(ctx, containerId)
			},
		},
	})
	informer.Run(stopper)
}

func getHostLogDir(ctx context.Context, containerId string) string {

	rt := ""

	docker_c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err.Error())
	}

	logDestination := "/busybox-data"

	containerJSON, err := docker_c.ContainerInspect(ctx, containerId)
	if err != nil {
		panic(err.Error())
	}
	for _, m := range containerJSON.Mounts {
		fmt.Printf("the container mount source is %s and destination is %s\n",
			m.Source, m.Destination)

		if m.Destination == logDestination {
			fmt.Printf("the host log dir is %s\n", m.Source)
			rt = m.Source
		}
	}

	return rt
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
