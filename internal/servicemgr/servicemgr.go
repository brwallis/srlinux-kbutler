package servicemgr

import (
	"fmt"
	"time"

	"github.com/brwallis/srlinux-kbutler/internal/agent"

	log "k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	yangRoot = ".kbutler"
)

var (
	KButler *agent.Agent
)

// ServiceController struct
type ServiceController struct {
	informerFactory informers.SharedInformerFactory
	serviceInformer coreinformers.ServiceInformer
}

// processService processes updates to Services
func processService(service *v1.Service) {
	log.Infof("Processing service... but not doing anything right now :): %s", service.Data)
}

// Run starts shared informers and waits for the shared informer cache to synchronize
func (c *ServiceController) Run(stopCh chan struct{}) error {
	// Starts all the shared informers that have been created by the factory so far
	c.informerFactory.Start(stopCh)
	// wait for the initial synchronization of the local cache
	if !cache.WaitForCacheSync(stopCh, c.serviceInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

func (c *ServiceController) serviceAdd(obj interface{}) {
	service := obj.(*v1.Service)
	log.Infof("Service CREATED: %s/%s", service.Namespace, service.Name)
	if service.Namespace == "kube-system" {
		if service.Name == "srlinux-config" {
			log.Infof("Service has the correct name: %s", service.Name)
			processService(service)
		}
	}
}

func (c *ServiceController) serviceUpdate(old, new interface{}) {
	oldService := old.(*v1.Service)
	newService := new.(*v1.Service)
	log.Infof(
		"Service UPDATED. %s/%s %s",
		oldService.Namespace, oldService.Name, newService.Name,
	)
	if newService.Namespace == "kube-system" {
		if newService.Name == "srlinux-config" {
			log.Infof("Service has the correct name: %s", newService.Name)
			processService(newService)
		}
	}
}

func (c *ServiceController) serviceDelete(obj interface{}) {
	service := obj.(*v1.Service)
	log.Infof("Service DELETED: %s/%s", service.Namespace, service.Name)
}

// NewServiceController creates a ServiceController
func NewServiceController(informerFactory informers.SharedInformerFactory) *ServiceController {
	serviceInformer := informerFactory.Core().V1().ConfigMaps()

	c := &ServiceController{
		informerFactory: informerFactory,
		serviceInformer: serviceInformer,
	}
	serviceInformer.Informer().AddEventHandler(
		// Your custom resource event handlers.
		cache.ResourceEventHandlerFuncs{
			// Called on creation
			AddFunc: c.serviceAdd,
			// Called on resource update and every resyncPeriod on existing resources.
			UpdateFunc: c.serviceUpdate,
			// Called on resource deletion.
			DeleteFunc: c.serviceDelete,
		},
	)
	return c
}

// ServiceMgr manages updates of Services from K8
func ServiceMgr(clientSet *kubernetes.Clientset, kButler *agent.Agent) {
	KButler = kButler
	informerFactory := informers.NewSharedInformerFactory(clientSet, time.Hour*24)
	controller := NewServiceController(informerFactory)

	stop := make(chan struct{})
	defer close(stop)
	err := controller.Run(stop)
	if err != nil {
		log.Fatal(err)
	}
	select {}
}
