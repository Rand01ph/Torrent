package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	dockerclient "github.com/docker/docker/client"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformersV1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelistersV1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type Controller struct {
	kubeclientset kubernetes.Interface
	workqueue     workqueue.RateLimitingInterface
	informer      cache.SharedIndexInformer
	podsLister    corelistersV1.PodLister
	podsSynced    cache.InformerSynced
	// docker client
	dockerClient *dockerclient.Client
}

func NewController(kubeclientset kubernetes.Interface, podInformer coreinformersV1.PodInformer) *Controller {

	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.WithVersion(dockerClientVersion))
	if err != nil {
		panic(err.Error())
	}

	controller := &Controller{
		kubeclientset: kubeclientset,
		podsLister:    podInformer.Lister(),
		podsSynced:    podInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Torrent"),
		dockerClient:  dockerClient,
	}

	klog.Info("Setting up event handlers")

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*coreV1.Pod)
			newPod := newObj.(*coreV1.Pod)
			if oldPod.ResourceVersion == newPod.ResourceVersion {
				return
			}
			controller.enqueuePod(newObj)
		},
		DeleteFunc: controller.enqueuePodDelete,
	})

	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {

	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Pod controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch threadiness workers to process Pod resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// 主要业务逻辑
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Pod resource with this namespace/name
	pod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
		// The Pod resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			klog.Infof("Pod does not exist in local cache: %s/%s", namespace, name)
			return nil
		}
		return err
	}

	// Get Torrent module_name and filter Container by it
	moduleName := pod.Annotations[moduleNameTag]
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == moduleName {
			containerID := strings.TrimPrefix(container.ContainerID, "docker://")
			// torrent/log_path: "nginx:/busybox-data:*.log;pro:/var/log:pro.log"
			if v, ok := pod.Annotations[logPathTag]; ok {
				logPaths := strings.Split(v, ";")
				for _, l := range logPaths {
					logDetails := strings.Split(l, ":")
					if len(logDetails) == 2 {
						logDir, logFile := path.Split(logDetails[1])
						c.getHostLogDir(containerID, logDetails[0], logDir, logFile, moduleName)
					}
					if len(logDetails) == 3 {
						c.getHostLogDir(containerID, logDetails[0], logDetails[1], logDetails[2], moduleName)
					}
				}
			}
		}
	}

	return nil
}

func (c *Controller) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) enqueuePodDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	if err = deleteLog(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func deleteLog(obj interface{}) error {
	pod := obj.(*coreV1.Pod)
	klog.Infof("Pod is deleted in local cache, begin delete it's log from Torrent")
	// Get Torrent module_name and filter Container by it
	moduleName := pod.Annotations[moduleNameTag]
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == moduleName {
			containerID := strings.TrimPrefix(container.ContainerID, "docker://")
			if containerID == "" {
				return fmt.Errorf("failed to find container id %v", containerID)
			}
			klog.Infof("Pod %s Deleted and container id is %s", pod.Name, containerID)
			files, err := filepath.Glob("/tmp/" + containerID + "_*.yml")
			if err != nil {
				return fmt.Errorf("find delete config error is %v", err)
			}
			for _, f := range files {
				if err := os.Remove(f); err != nil {
					return fmt.Errorf("delete %s config error is %v", f, err)
				}
			}
			klog.Infof("delete pod(%s) log config success", pod.Name)
			return nil
		}
	}
	return nil
}

func (c *Controller) getHostLogDir(containerId string, logType string, logDestination string, logFiles string, moduleName string) string {
	rt := ""
	containerJSON, err := c.dockerClient.ContainerInspect(context.Background(), containerId)
	if err != nil {
		klog.Error(err.Error())
		return ""
	}
	for _, m := range containerJSON.Mounts {
		klog.Infof("the container mount source is %s and destination is %s\n", m.Source, m.Destination)

		if m.Destination == logDestination {
			klog.Infof("the container %s log dir is %s\n", containerId, m.Source)
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

			logInputConfig := fmt.Sprintf("/tmp/%s_%s.yml", containerId, logType)
			f, err := os.Create(logInputConfig)
			if err != nil {
				panic(err.Error())
			}

			if err := templ.Execute(f, config); err != nil {
				panic(err.Error())
			}

			if err := f.Close(); err != nil {
				panic(err.Error())
			}
			klog.Infof("create log input config %s success", logInputConfig)
		}
	}
	return rt
}
