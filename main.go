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
	"io"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"os"
	"path"
	"path/filepath"
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
	inputTemplatePath = "filebeat-input-log.tpl"
	moduleNameTag = "torrent/module_name"
	logPathTag = "torrent/log_path"
)

func main() {

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

	// copy configmap yaml
	srcDir := "/etc/config/input"
	dstDir := "/tmp"
	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		fmt.Println(err)
	}
	if len(files) == 0 {
		fmt.Println("input 文件夹没有找到配置文件!!!")
	}
	for _, f := range files {
		fmt.Printf("开始Copy input 配置文件 %v !!!\n", f.Name())
		filePath := srcDir + string(filepath.Separator) + f.Name()
		if filepath.Ext(filePath) == "yml" {
			dstPath := strings.Replace(f.Name(), srcDir, dstDir, 1)
			copyFile(filePath, dstPath)
		}
	}

	ctx := context.Background()

	factory := informers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Core().V1().Pods().Informer()
	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			mObj := obj.(*corev1.Pod)
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
				if v, ok := mObj.Annotations[moduleNameTag]; ok {
					moduleName = v
				}

				// torrent/log_path: "nginx:/busybox-data:*.log;pro:/var/log:pro.log"
				if v, ok := mObj.Annotations[logPathTag]; ok {
					logPaths := strings.Split(v, ";")
					for _, l := range logPaths {
						logDetails := strings.Split(l, ":")
						if len(logDetails) == 2 {
							logdir, logfile := path.Split(logDetails[1])
							go getHostLogDir(ctx, containerId, logDetails[0], logdir, logfile, moduleName)
						}
						if len(logDetails) == 3 {
							go getHostLogDir(ctx, containerId, logDetails[0], logDetails[1], logDetails[2], moduleName)
						}
					}
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
		fmt.Println(err.Error())
		return ""
	}
	for _, m := range containerJSON.Mounts {
		fmt.Printf("the container mount source is %s and destination is %s\n",
			m.Source, m.Destination)

		if m.Destination == logDestination {
			fmt.Printf("the container %s log dir is %s\n", containerId, m.Source)
			rt = "/host" + m.Source

			templ, err := template.ParseFiles(inputTemplatePath)
			if err != nil {
				panic(err.Error())
			}

			config := map[string]string{
				"logPath":    rt,
				"moduleName": moduleName,
				"logFiles":   logFiles,
				"logType":    logType,
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

func copyFile(srcFile, destFile string) error {
	file, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer file.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer dest.Close()

	io.Copy(dest, file)

	return nil
}
