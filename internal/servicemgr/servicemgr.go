package servicemgr

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brwallis/srlinux-kbutler/internal/agent"
	"github.com/brwallis/srlinux-kbutler/internal/config"

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

// func (ay *AgentYang) AddService(
// 	namespace string,
// 	serviceName string,
// 	externalAddress string,
// 	node string,
// 	nodeAddress string,
// 	nextHops []string) {
// 	// Build structs for the service
// 	newAddress := Address{
// 		Value: nodeAddress,
// 	}
// 	newNode := Node{
// 		Address:  newAddress,
// 		// Hostname: node,
// 		// Hostname: Name{
// 		// 	Value: node,
// 		// },
// 		// NextHop:
// 	}
// 	nodes := make(map[string]Node)
// 	nodes[node] = newNode

// 	newExternalAddress := ExternalAddress{
// 		Node:    nodes,
// 		// Address: externalAddress,
// 		// Address: Address{
// 		// 	Value: externalAddress,
// 		// },
// 	}
// 	externalAddresses := make(map[string]ExternalAddress)
// 	externalAddresses[externalAddress] = newExternalAddress

// 	newService := Service{
// 		ExternalAddress: externalAddresses,
// 		// Name:            name,
// 		// Name: Name{
// 		// 	Value: name,
// 		// },
// 	}
// 	services := make(map[string]Service)
// 	services[name] = newService

// 	newNamespace := Namespace{
// 		Service: services,
// 		// Name:    namespace,
// 		// Name: Name{
// 		// 	Value: namespace,
// 		// },
// 	}
// 	namespaces := make(map[string]Namespace)
// 	namespaces[namespace] = newNamespace
// }

// processService processes updates to Services
func processService(service *v1.Service) {
	var serviceYang config.Service
	// var externalAddressYang config.ExternalAddress
	log.Infof("Processing service... Service name: %s", service.Name)

	jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}", yangRoot, service.Name, service.Namespace)
	serviceYang.OperState.Value = "up"
	serviceData, err := json.Marshal(serviceYang)
	if err != nil {
		log.Infof("Failed to marshal data for service: %v", err)
	}
	serviceString := string(serviceData)
	KButler.UpdateServiceTelemetry(&jsPath, &serviceString)

	// KButler.Yang.AddService(service.Namespace, service.Name, service.Spec.ClusterIP, "batman", "batman", []string{"test", "test"})

	// log.Infof("Processing service external address... Service name: %s, address: %s", service.Name, service.Spec.ClusterIP)
	// externalAddressPath := fmt.Sprintf("%s.external_address{.address==\"%s\"&&.hostname==\"%s\"}", jsPath, service.Spec.ClusterIP, "batman")
	// externalAddressYang.HostAddress.Value = "192.168.0.14"
	// service.
	// service
	// externalAddressData, err := json.Marshal(externalAddressYang)
	// if err != nil {
	// 	log.Infof("Failed to marshal data for service: %v", err)
	// }
	// externalAddressString := string(externalAddressData)
	// KButler.UpdateServiceTelemetry(&externalAddressPath, &externalAddressString)
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
