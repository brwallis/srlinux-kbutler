package endpointmgr

import (
	"context"
	"encoding/json"
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

// EndpointController struct
type EndpointController struct {
	informerFactory  informers.SharedInformerFactory
	endpointInformer coreinformers.EndpointsInformer
}

// getIPFromNodeName takes a node name and a K8s clientset and queries the API server for the internalIP of the node
func getIPFromNodeName(nodeName string) string {
	var nodeAddress v1.NodeAddress
	node, err := ClientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error retrieving node with name: %s, %e", nodeName, err)
	}

	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			nodeAddress = address
		} else {
			log.Infof("Skipping adding address: %s, of type: %s", nodeAddress.Address, nodeAddress.Type)
		}
	}
	return nodeAddress.Address
}

// processEndpoint processes updates to Endpoints
func processEndpoint(endpoint *v1.Endpoints) {
	// Get the service this endp
	// var endpointYang config.Service
	// var externalAddressYang config.ExternalAddress
	log.Infof("Processing endpoint... Endpoint name: %s, subsets: %v", endpoint.Name, endpoint.Subsets, endpoint.Subsets)
	// Iterate over the subsets
	for _, endpointlist := range endpoint.Subsets {
		log.Infof("Found endpoint list for service: %s: %#v", endpoint.Name, endpointlist)
		// Iterate over each address
		log.Infof("Iterating over addresses for service: %s...", endpoint.Name)
		for _, addresses := range endpointlist.Addresses {
			nodeName := addresses.NodeName
			// Check if nodename is a valid ptr
			if nodeName == nil {
				log.Infof("No valid nodename found for service: %s, address: %v", endpoint.Name, addresses.IP)
			} else {
				var externalAddressYang config.ExternalAddress

				jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}.external_address{.address==\"%s\"&&.hostname==\"%s\"}", yangRoot, endpoint.Name, endpoint.Namespace, addresses.IP, *nodeName)
				nodeAddress := getIPFromNodeName(*nodeName)
				externalAddressYang.HostAddress.Value = nodeAddress
				eaData, err := json.Marshal(externalAddressYang)
				if err != nil {
					log.Infof("Failed to marshal data for endpoint: %v", err)
				}
				eaString := string(eaData)
				KButler.UpdateServiceTelemetry(&jsPath, &eaString)
				log.Infof("Processing address: %v, node name: %s, for service: %s...", addresses.IP, *nodeName, endpoint.Name)
			}
		}
		log.Infof("%s", endpointlist.Addresses)
	}

	// jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}", yangRoot, service.Name, service.Namespace)
	// serviceYang.OperState.Value = "up"
	// serviceData, err := json.Marshal(serviceYang)
	// if err != nil {
	// 	log.Infof("Failed to marshal data for service: %v", err)
	// }
	// serviceString := string(serviceData)
	// KButler.UpdateServiceTelemetry(&jsPath, &serviceString)

	// // KButler.Yang.AddService(service.Namespace, service.Name, service.Spec.ClusterIP, "batman", "batman", []string{"test", "test"})

	// log.Infof("Processing service external address... Service name: %s, address: %s", service.Name, service.Spec.ClusterIP)
	// externalAddressPath := fmt.Sprintf("%s.external_address{.address==\"%s\"&&.hostname==\"%s\"}", jsPath, service.Spec.ClusterIP, "batman")
	// externalAddressYang.HostAddress.Value = "192.168.0.14"
	// externalAddressData, err := json.Marshal(externalAddressYang)
	// if err != nil {
	// 	log.Infof("Failed to marshal data for service: %v", err)
	// }
	// externalAddressString := string(externalAddressData)
	// KButler.UpdateServiceTelemetry(&externalAddressPath, &externalAddressString)
}

// Run starts shared informers and waits for the shared informer cache to synchronize
func (c *EndpointController) Run(stopCh chan struct{}) error {
	// Starts all the shared informers that have been created by the factory so far
	c.informerFactory.Start(stopCh)
	// wait for the initial synchronization of the local cache
	if !cache.WaitForCacheSync(stopCh, c.endpointInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

func (c *EndpointController) endpointAdd(obj interface{}) {
	endpoint := obj.(*v1.Endpoints)
	log.Infof("Endpoint CREATED: %s/%s", endpoint.Namespace, endpoint.Name)
	// log.Infof("Endpoint %s/%s has ClusterIP: %v, ClusterIP/s: %v, ExternalIP/s: %v", endpoint.Namespace, endpoint.Name, endpoint.Spec.ClusterIP, endpoint.Spec.ClusterIPs, service.Spec.ExternalIPs)
	processEndpoint(endpoint)
	if endpoint.Namespace == "kube-system" {
		if endpoint.Name == "srlinux-config" {
			log.Infof("Endpoint has the correct name: %s", endpoint.Name)
			processEndpoint(endpoint)
		}
	}
}

func (c *EndpointController) endpointUpdate(old, new interface{}) {
	oldEndpoint := old.(*v1.Endpoints)
	newEndpoint := new.(*v1.Endpoints)
	log.Infof(
		"Endpoint UPDATED. %s/%s %s",
		oldEndpoint.Namespace, oldEndpoint.Name, newEndpoint.Name,
	)
	if newEndpoint.Namespace == "kube-system" {
		if newEndpoint.Name == "srlinux-config" {
			log.Infof("Endpoint has the correct name: %s", newEndpoint.Name)
			processEndpoint(newEndpoint)
		}
	}
}

func (c *EndpointController) endpointDelete(obj interface{}) {
	endpoint := obj.(*v1.Endpoints)
	log.Infof("Endpoint DELETED: %s/%s", endpoint.Namespace, endpoint.Name)
}

// NewEndpointController creates a EndpointController
func NewEndpointController(informerFactory informers.SharedInformerFactory) *EndpointController {
	endpointInformer := informerFactory.Core().V1().Endpoints()

	c := &EndpointController{
		informerFactory:  informerFactory,
		endpointInformer: endpointInformer,
	}
	endpointInformer.Informer().AddEventHandler(
		// Your custom resource event handlers.
		cache.ResourceEventHandlerFuncs{
			// Called on creation
			AddFunc: c.endpointAdd,
			// Called on resource update and every resyncPeriod on existing resources.
			UpdateFunc: c.endpointUpdate,
			// Called on resource deletion.
			DeleteFunc: c.endpointDelete,
		},
	)
	return c
}

// EndpointMgr manages updates of Endpoints from K8
func EndpointMgr(clientSet *kubernetes.Clientset, kButler *agent.Agent) {
	KButler = kButler
	ClientSet = clientSet
	informerFactory := informers.NewSharedInformerFactory(clientSet, time.Hour*24)
	controller := NewEndpointController(informerFactory)

	stop := make(chan struct{})
	defer close(stop)
	err := controller.Run(stop)
	if err != nil {
		log.Fatal(err)
	}
	select {}
}
