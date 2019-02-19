/*
k8s标签生成log收集配置文件

1. k8s标签->Pods
2. Pods->ContainerIds
3. ContainerIds->logHostPath
4. logHostPath->配置文件

*/

package main

import (
	"fmt"
	"k8s.io/client-go/rest"
	"os"
	"strings"
	"text/template"

	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (

)

func main() {

	//var kubeconfig *string
	//if home := homeDir(); home != "" {
	//	kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	//} else {
	//	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	//}
	//flag.Parse()
	//
	//// use the current context in kubeconfig
	//config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	//if err != nil {
	//	panic(err.Error())
	//}
	//
	//// create the clientset
	//clientset, err := kubernetes.NewForConfig(config)
	//if err != nil {
	//	panic(err.Error())
	//}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
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

				moduleName := ""
				// 获取annotations中的module_name以及log_path
				if v, ok := mObj.Annotations["torrent/module_name"]; ok {
					fmt.Println(v)
					moduleName = v
				} else {
					fmt.Println("Module Name Not Found")
				}

				// torrent/log_path: "nginx:/busybox-data:*.log;pro:/var/log:pro.log"
				if v, ok := mObj.Annotations["torrent/log_path"]; ok {
					fmt.Println(v)
					logPaths := strings.Split(v, ";")
					for _, l := range logPaths {
						logDetails := strings.Split(l, ":")
						go getHostLogDir(ctx, containerId, logDetails[0], logDetails[1], logDetails[2], moduleName)
					}
				} else {
					fmt.Println("Log Path Not Found")
				}
			},
		},
	})
	informer.Run(stopper)
}

func getHostLogDir(ctx context.Context, containerId string, logType string, logDestination string, logFiles string, moduleName string) string {

	rt := ""

	dockerC, err := client.NewClientWithOpts(client.WithVersion("1.38"))
	//dockerC, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err.Error())
	}

	containerJSON, err := dockerC.ContainerInspect(ctx, containerId)
	if err != nil {
		panic(err.Error())
	}
	for _, m := range containerJSON.Mounts {
		fmt.Printf("the container mount source is %s and destination is %s\n",
			m.Source, m.Destination)

		if m.Destination == logDestination {
			fmt.Printf("the container %s log dir is %s\n",containerId, m.Source)
			rt = "/host" + m.Source

			templ, err := template.ParseFiles("filebeat-input-log.tpl")
			if err != nil {
				panic(err.Error())
			}

			config := map[string]string{
				"logPath": rt,
				"moduleName": moduleName,
				"logFiles": logFiles,
				"logType": logType,
			}

			f, err := os.Create("/tmp/" + containerId + "_" + logType + ".yml")
			if err != nil {
				panic(err.Error())
			}

			if err := templ.Execute(f, config); err != nil {
				panic(err.Error())
			}

			if err := f.Close(); err != nil {
				panic(err.Error())
			}
		}
	}

	return rt
}

//func homeDir() string {
//	if h := os.Getenv("HOME"); h != "" {
//		return h
//	}
//	return os.Getenv("USERPROFILE") // windows
//}
