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
    "log"
    "os"
    "path"
    "path/filepath"
    "strings"
    "text/template"

    "github.com/docker/docker/client"
    "github.com/fsnotify/fsnotify"
    "golang.org/x/net/context"
    coreV1 "k8s.io/api/core/v1"
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/cache"
)

const (
    moduleNameTag     = "torrent/module_name"
    logPathTag        = "torrent/log_path"
    srcDir            = "/etc/config/input"
    dstDir            = "/tmp"
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

    // creates the in-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
        panic(err.Error())
    }
    // creates the clientSet
    clientSet, err := kubernetes.NewForConfig(config)
    if err != nil {
        panic(err.Error())
    }

    copyConfigMap(srcDir, dstDir)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // auto reload configmap for filebeat
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Fatal(err)
    }
    defer watcher.Close()

    go func(ctx context.Context) {
        for {
            select {
            case event := <-watcher.Events:
                if event.Op&fsnotify.Create == fsnotify.Create {
                    if filepath.Base(event.Name) == "..data" {
                        log.Println("config map updated")
                        copyConfigMap(srcDir, dstDir)
                    }
                }
            case err := <-watcher.Errors:
                log.Println("error:", err)
            case <-ctx.Done():
                log.Println("controller exit...")
                return
            }
        }
    }(ctx)
    err = watcher.Add(srcDir)
    if err != nil {
        log.Fatal(err)
    }

    factory := informers.NewSharedInformerFactory(clientSet, 0)
    informer := factory.Core().V1().Pods().Informer()
    stopper := make(chan struct{})
    defer close(stopper)
    informer.AddEventHandler(cache.FilteringResourceEventHandler{
        FilterFunc: func(obj interface{}) bool {
            // 当 pod 被删除时， obj 可以是 *v1.Pod 或者 DeletionFinalStateUnknown marker item.
            mObj, ok := obj.(*coreV1.Pod)
            if ok && mObj.Status.Phase == coreV1.PodRunning {
                return true
            }
            return false
        },
        Handler: cache.ResourceEventHandlerFuncs{
            AddFunc:    addPodLog,
            UpdateFunc: updatePodLog,
            DeleteFunc: deletePodLog,
        },
    })
    informer.Run(stopper)
}

func addPodLog(obj interface{}) {
    mObj := obj.(*coreV1.Pod)
    containerID := strings.TrimPrefix(mObj.Status.ContainerStatuses[0].ContainerID, "docker://")
    if containerID == "" {
        log.Printf("Failed to find container id %v", containerID)
        return
    }
    fmt.Printf("New Pod %s Added to Store \t container id is %s\n", mObj.Name, containerID)
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
                logDir, logFile := path.Split(logDetails[1])
                go getHostLogDir(containerID, logDetails[0], logDir, logFile, moduleName)
            }
            if len(logDetails) == 3 {
                go getHostLogDir(containerID, logDetails[0], logDetails[1], logDetails[2], moduleName)
            }
        }
    }
}

func updatePodLog(oldObj, newObj interface{}) {
    oObj := oldObj.(*coreV1.Pod)
    oldContainerID := strings.TrimPrefix(oObj.Status.ContainerStatuses[0].ContainerID, "docker://")
    if oldContainerID == "" {
        log.Printf("Failed to find container id %v", oldContainerID)
        return
    }
    nObj := newObj.(*coreV1.Pod)
    newContainerID := strings.TrimPrefix(nObj.Status.ContainerStatuses[0].ContainerID, "docker://")
    if newContainerID == "" {
        log.Printf("Failed to find container id %v", newContainerID)
        return
    }
    log.Printf("old pod %s and container id %s update ====> new pod %s and container id %s", oObj.Name, oldContainerID, nObj.Name, newContainerID)
}

func deletePodLog(obj interface{}) {
    mObj := obj.(*coreV1.Pod)
    containerID := strings.TrimPrefix(mObj.Status.ContainerStatuses[0].ContainerID, "docker://")
    if containerID == "" {
        log.Printf("Failed to find container id %v", containerID)
        return
    }
    log.Printf("Pod %s Deleted and container id is %s", mObj.Name, containerID)
    files, err := filepath.Glob("/tmp/" + containerID + "_*.yml")
    if err != nil {
        log.Printf("find delete config error is %v", err)
    }
    for _, f := range files {
        if err := os.Remove(f); err != nil {
            log.Printf("delete %s config error is %v", f, err)
        }
    }
}

func getHostLogDir(containerId string, logType string, logDestination string, logFiles string, moduleName string) string {
    rt := ""
    dockerClient, err := client.NewClientWithOpts(client.WithVersion(dockerClientVersion))
    //dockerC, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        panic(err.Error())
    }
    containerJSON, err := dockerClient.ContainerInspect(context.Background(), containerId)
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

func copyConfigMap(srcDir, dstDir string) error {
    // copy configmap yaml
    log.Println("begein to copy configmap")
    files, err := ioutil.ReadDir(srcDir)
    if err != nil {
        log.Println(err)
    }
    if len(files) == 0 {
        log.Println("input dir has no config file!!!")
    }
    for _, f := range files {
        filePath := srcDir + string(filepath.Separator) + f.Name()
        log.Printf("config path is %v", filePath)
        if filepath.Ext(filePath) == ".yml" {
            dstPath := strings.Replace(filePath, srcDir, dstDir, 1)
            log.Printf("Copy input config %v to %v !!!\n", filePath, dstPath)
            copyFile(filePath, dstPath)
        } else {
            log.Println("no yml config")
        }
    }
    return nil
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
