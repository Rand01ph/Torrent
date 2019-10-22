package main

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	coreinformersV1 "k8s.io/client-go/informers/core/v1"
	corelistersV1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

type Controller struct {
	kubeclientset kubernetes.Interface
	workqueue workqueue.RateLimitingInterface
	informer     cache.SharedIndexInformer
	podsLister corelistersV1.PodLister
	podsSynced cache.InformerSynced
	// docker client
}


func NewController(kubeclientset kubernetes.Interface, podInformer coreinformersV1.PodInformer) *Controller {

	controller := &Controller{
		kubeclientset: kubeclientset,
		podsLister: podInformer.Lister(),
		podsSynced: podInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Torrent"),
	}

	klog.Info("Setting up event handlers")
	
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})


	return controller
}