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
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

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

	docker_c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err.Error())
	}

	for {

		namespace := "default"
		pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		logDestination := "/busybox-data"

		for _, d := range pods.Items {
			containerId := d.Status.ContainerStatuses[0].ContainerID[9:]
			fmt.Printf("The pod %s containerID is %s\n", d.Name, containerId)
			containerJSON, err := docker_c.ContainerInspect(ctx, containerId)
			if err != nil {
				panic(err.Error())
			}
			for _, m := range containerJSON.Mounts {
				fmt.Printf("the container mount source is %s and destination is %s\n",
					m.Source, m.Destination)

				if m.Destination == logDestination {
					fmt.Printf("the host log dir is %s\n", m.Source)
				}
			}
		}

		time.Sleep(10 * time.Second)
	}
}


func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
