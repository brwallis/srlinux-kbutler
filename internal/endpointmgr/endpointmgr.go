package endpointmgr

import (
	"context"
	"fmt"
	"time"

	"github.com/brwallis/srlinux-go/pkg/gnmi"
	srlyangrelease "github.com/brwallis/srlinux-go/pkg/yangrelease"
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

// getExternalIPForService takes a service name and namespace, and returns the external IP address
func getExternalIPForService(serviceName string, namespace string) string {
	var externalAddress string
	service, err := ClientSet.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error retrieving service with name: %s, %e", serviceName, err)
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		externalAddress = ingress.IP
	}
	return externalAddress
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

// processDeltas takes a list of endpoints for a service, and compares it to the previous list stored, deleting any entries that no longer exist
func processDeltas(newEndpoints []agent.EndpointKey, service agent.ServiceKey) {
	// Iterate over the old endpoints, comparing to the new
	log.Infof("Looping over old endpoint list...")
	for _, oldEndpoint := range KButler.ServiceMap[service] {
		log.Infof("Checking old endpoint %v...", oldEndpoint)
		endpointMatched := false
		for _, newEndpoint := range newEndpoints {
			log.Infof("Checking old endpoint: %v, against new endpoint: %v", oldEndpoint, newEndpoint)
			if newEndpoint == oldEndpoint {
				log.Infof("Old endpoint: %v, matches new endpoint!: %v", oldEndpoint, newEndpoint)
				endpointMatched = true
			}
		}
		if !endpointMatched {
			log.Infof("Unable to find match for old endpoint: %v, deleting", oldEndpoint)
			KButler.DeleteEndpoint(service, oldEndpoint)
		}
	}
	KButler.ServiceMap[service] = newEndpoints
}

// processEndpoint processes adds/updates to Endpoints
func processEndpoint(endpoint *v1.Endpoints) {
	var serviceData config.Service
	var externalRouteMatched bool
	var externalRouteProgrammed bool
	var nodeRouteMatched bool
	var nodeRouteUnmatched bool
	var externalAddress string
	log.Infof("Processing endpoint... Endpoint name: %s, subsets: %v", endpoint.Name, endpoint.Subsets, endpoint.Subsets)
	externalAddress = getExternalIPForService(endpoint.Name, endpoint.Namespace)
	serviceKey := agent.ServiceKey{Name: endpoint.Name, Namespace: endpoint.Namespace}
	// We don't want to process services that do not have external addresses
	if externalAddress != "" {
		var currentEndpoints []agent.EndpointKey
		for _, endpointlist := range endpoint.Subsets {
			for _, address := range endpointlist.Addresses {
				nodeName := address.NodeName
				// Check if nodename is a valid ptr
				if nodeName == nil {
					log.Infof("No valid nodename found for service: %s, address: %v", endpoint.Name, address.IP)
				} else {
					var endpointData config.Endpoint
					endpointKey := agent.EndpointKey{ExternalAddress: externalAddress, Hostname: *nodeName}
					currentEndpoints = append(currentEndpoints, endpointKey)
					log.Infof("Processing address: %v, node name: %s, for service: %s...", address.IP, *nodeName, endpoint.Name)
					resp, err := gnmi.Get("/network-instance[name=default]/route-table")
					if err != nil {
						log.Infof("Received error while getting route table for endpoint: %s", endpoint.Name)
						continue
					}
					dev := srlyangrelease.SrlNokiaNetworkInstance_NetworkInstance_RouteTable{}
					err = srlyangrelease.Unmarshal(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal(), &dev)
					if err != nil {
						log.Infof("Received error while unmarshaling get response: %e", err)
						continue
					}
					// log.Infof("Unmarshaled get response: %#v", dev)
					// Ensure we have a valid route for the external address
					nodeAddress := getIPFromNodeName(*nodeName)
					for routekey, route := range dev.Ipv4Unicast.Route {
						log.Infof("Iterating over route: %s, key: %s", *route.Ipv4Prefix, routekey)
						// All routes will have a /32 prefix, and this is what is stored, so we need to match on that
						routePrefix := fmt.Sprintf("%s/32", externalAddress)
						// Check if the route matches
						if *route.Ipv4Prefix == routePrefix {
							externalRouteMatched = true
							log.Infof("Got a match for external address: %s, prefix: %s, nexthops: %v", externalAddress, *route.Ipv4Prefix, *route.NextHopGroup)
							// Check if the route is programmed
							if route.FibProgramming.Status.String() == "success" {
								externalRouteProgrammed = true
								// Grab the next hops from the nexthopgroup
								nextHopGroup := dev.NextHopGroup[*route.NextHopGroup]
								for _, nextHop := range nextHopGroup.NextHop {
									log.Infof("Found next hop for prefix: %s, via nexthopgroup: %s. Next hop: %s", routePrefix, *route.NextHopGroup, *nextHop.NextHop)
									nextHopObj := dev.NextHop[*nextHop.NextHop]
									log.Infof("Nexthop %s for prefix %s resolves to address: %s", *nextHop.NextHop, routePrefix, *nextHopObj.IpAddress)
									if *nextHopObj.IpAddress == nodeAddress {
										nodeRouteMatched = true
									}
								}
								if nodeRouteMatched {
									log.Infof("Node address %s is a valid next hop for external address %s, publishing oper-state up!", nodeAddress, routePrefix)
									endpointData.HostAddress.Value = nodeAddress
									endpointData.OperState.Value = "up"
									endpointData.FIBProgrammed.Value = true
								} else {
									nodeRouteUnmatched = true
									log.Infof("Node address %s is NOT a valid next hop for external address %s, publishing oper-state down!", nodeAddress, routePrefix)
									endpointData.HostAddress.Value = nodeAddress
									endpointData.OperState.Value = "down"
									endpointData.OperReason.Value = "no-route-to-host"
									endpointData.FIBProgrammed.Value = false
								}
								KButler.YangEndpoint[endpointKey] = &endpointData
								KButler.UpdateEndpointTelemetry(serviceKey, endpointKey)
							} else {
								log.Infof("External route: %s is not programmed in the FIB", routePrefix)
								externalRouteProgrammed = false
							}
						}
					}
				}
			}
			// Clean up removed endpoints
			log.Infof("Cleaning up endpoint list, new endpoints: %v, old endpoints: %v", currentEndpoints, KButler.ServiceMap[serviceKey])
			processDeltas(currentEndpoints, serviceKey)
			// Process service updates
			if externalRouteMatched {
				// If we did, the service is either up or degraded
				if externalRouteProgrammed {
					if nodeRouteUnmatched {
						log.Infof("External address %s routable, but not all nodes are present, publishing oper-state degraded!", externalAddress)
						serviceData.OperState.Value = "degraded"
						serviceData.OperReason.Value = "endpoint-nexthop-missing"
					} else {
						log.Infof("External address %s routable, and all nodes available, publishing oper-state up!", externalAddress)
						serviceData.OperState.Value = "up"
						serviceData.OperReason.Value = ""
					}
				} else {
					serviceData.OperState.Value = "down"
					serviceData.OperReason.Value = "external-address-not-programmed"
				}
			} else {
				serviceData.OperState.Value = "down"
				serviceData.OperReason.Value = "external-address-no-route"
			}
			KButler.YangService[serviceKey] = &serviceData
			KButler.UpdateServiceTelemetry(serviceKey)
		}
	} else {
		log.Infof("Skipping processing for service: %s - no external IPs", endpoint.Name)
	}
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
	processEndpoint(newEndpoint)
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
