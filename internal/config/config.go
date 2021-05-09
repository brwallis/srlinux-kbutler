package config

type Address struct {
	Value string `json:"value"`
}

type Node struct {
	Address Address `json:"address"`
	// NextHop struct {
	// 	Value string `json:"value"`
	// } `json:"next_hop"`
}

type ExternalAddress struct {
	Node map[string]Node `json:"node"`
	// NextHops struct {
	// 	value []string `json:"next_hops"`
	// }
}

type Service struct {
	ExternalAddress map[string]ExternalAddress `json:"external_address"`
}

type Namespace struct {
	Service map[string]Service `json:"service"`
}

// AgentYang holds the YANG schema for the agent
type AgentYang struct {
	Namespace map[string]Namespace `json:"namespace"`
}

func (ay *AgentYang) AddService(
	namespace string,
	name string,
	externalAddress string,
	node string,
	nodeAddress string,
	nextHops []string) {
	// Build structs for the service
	newAddress := Address{
		Value: nodeAddress,
	}
	newNode := Node{
		Address: newAddress,
		// NextHop:
	}
	nodes := make(map[string]Node)
	nodes[node] = newNode

	newExternalAddress := ExternalAddress{
		Node: nodes,
	}
	externalAddresses := make(map[string]ExternalAddress)
	externalAddresses[externalAddress] = newExternalAddress

	newService := Service{
		ExternalAddress: externalAddresses,
	}
	services := make(map[string]Service)
	services[name] = newService

	newNamespace := Namespace{
		Service: services,
	}
	namespaces := make(map[string]Namespace)
	namespaces[namespace] = newNamespace

	// Update telemetry
	ay.Namespace = namespaces
}

// func (ay *AgentYang) AddNamespace(name string, service struct{}) {
// 	_, ok := ay.Namespace[name]
// 	if !ok {
// 		ay.Namespace[name].Service = service
// 	}
// 	ay.Namespace = append(ay.Namespace, name)
// }
// func (ay *AgentYang) SetNodeName(name string) {
// 	ay.NodeName.Value = name
// }

// func (ay *AgentYang) GetNodeName() string {
// 	return ay.NodeName.Value
// }

// func (ay *AgentYang) SetName(name string) {
// 	ay.Name.Value = name
// }

// func (ay *AgentYang) GetName() string {
// 	return ay.Name.Value
// }

// func (ay *AgentYang) SetResponse(response string) {
// 	ay.Response.Value = response
// }

// func (ay *AgentYang) GetResponse() string {
// 	return ay.Response.Value
// }

// func (ay *AgentYang) SetCPU(cpuMin uint32, cpuMax uint32) {
// 	ay.CPUMin.Value = cpuMin
// 	ay.CPUMax.Value = cpuMax
// }

// func (ay *AgentYang) GetCPU() (cpuMin uint32, cpuMax uint32) {
// 	return ay.CPUMin.Value, ay.CPUMax.Value
// }

// func (ay *AgentYang) SetPods(cluster uint32, local uint32) {
// 	ay.ClusterPods.Value = cluster
// 	ay.LocalPods.Value = local
// }

// func (ay *AgentYang) GetPods() (cluster uint32, local uint32) {
// 	return ay.ClusterPods.Value, ay.LocalPods.Value
// }
