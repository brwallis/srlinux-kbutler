package servicemgr

import (
	"context"
	"fmt"
	"time"

	"github.com/brwallis/srlinux-kbutler/internal/agent"
	"github.com/brwallis/srlinux-kbutler/internal/config"

	log "k8s.io/klog"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	yangRoot = ".kbutler"
)

var (
	KButler   *agent.Agent
	ClientSet *kubernetes.Clientset
)

// ServiceController struct
type ServiceController struct {
	informerFactory informers.SharedInformerFactory
	serviceInformer coreinformers.ServiceInformer
}

// getExternalIPForService takes a service name and namespace, and returns the external IP address
func getExternalIPForService(serviceName string, namespace string) string {
	var externalAddress string
	service, err := ClientSet.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error retrieving service with name: %s, %e", serviceName, err)
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		externalAddress = ingress.IP
		// if address.Type == "InternalIP" {
		// 	externalAddress = address
		// } else {
		// 	log.Infof("Skipping adding address: %s, of type: %s", nodeAddress.Address, nodeAddress.Type)
		// }
	}
	return externalAddress
}

// processService processes updates to Services
func processService(service *v1.Service) {
	var serviceKey agent.ServiceKey
	var serviceData config.Service
	// var externalAddressYang config.ExternalAddress
	if service.Status.LoadBalancer.Ingress != nil {
		log.Infof("Processing service... Service name: %s", service.Name)

		// jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}", yangRoot, service.Name, service.Namespace)
		serviceKey.Name = service.Name
		serviceKey.Namespace = service.Namespace
		serviceData.OperState.Value = "updating"
		serviceData.OperReason.Value = "processing-service-update"
		KButler.YangService[serviceKey] = &serviceData
		KButler.UpdateServiceTelemetry(serviceKey)
		// serviceData, err := json.Marshal(serviceYang)
		// if err != nil {
		// 	log.Infof("Failed to marshal data for service: %v", err)
		// }
		// serviceString := string(serviceData)
		// KButler.UpdateServiceTelemetry(&jsPath, &serviceString)

	} else {
		log.Infof("Skipping processing service: %s, no external IP: %v", service.Name, service.Status.LoadBalancer.Ingress)
	}
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
	log.Infof("Service %s/%s has ClusterIP: %v, ClusterIP/s: %v, ExternalIP/s: %v", service.Namespace, service.Name, service.Spec.ClusterIP, service.Spec.ClusterIPs, service.Spec.ExternalIPs)

	processService(service)
	// config.AddService(service.Namespace, service.Name, externalAddress, node, nodeAddress, nextHops)
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
	// serviceInformer := informerFactory.Core().V1().ConfigMaps()
	serviceInformer := informerFactory.Core().V1().Services()

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
	ClientSet = clientSet

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
