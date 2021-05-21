package config

// import "fmt"

type ProgrammingState struct {
	Value bool `json:"value"`
}

type OperState struct {
	Value string `json:"value"`
}

type Address struct {
	Value string `json:"value"`
}

type Name struct {
	Value string `json:"value"`
}

type Node struct {
	Address Address `json:"address"`
	// Hostname Name    `json:"hostname"`
	// Hostname string `json:"hostname"`
	// NextHop struct {
	// 	Value string `json:"value"`
	// } `json:"next_hop"`
}

type Endpoint struct {
	// Node map[string]Node `json:"node"`
	OperState     OperState        `json:"oper_state"`
	OperReason    OperState        `json:"oper_reason"`
	FIBProgrammed ProgrammingState `json:"fib_programmed"`
	HostAddress   Address          `json:"host_address"`
	// Address Address         `json:"address"`
	// Address string `json:"address"`
	// NextHops struct {
	// 	value []string `json:"next_hops"`
	// }
}

type Service struct {
	// ExternalAddress map[string]ExternalAddress `json:"external_address"`
	OperState  OperState `json:"oper_state"`
	OperReason OperState `json:"oper_reason"`
	// Name            Name                       `json:"name"`
	// Name string `json:"name"`
}

type Namespace struct {
	Service map[string]Service `json:"service"`
	// Name    Name               `json:"name"`
	// Name string `json:"name"`
}

// AgentYang holds the YANG schema for the agent
type AgentYang struct {
	Controller Address `json:"controller"`
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
