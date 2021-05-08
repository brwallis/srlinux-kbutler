package config

// Yang holds the YANG schema for the agent
type AgentYang struct {
	Name struct {
		Value string `json:"value"`
	} `json:"name"`
	Response struct {
		Value string `json:"value"`
	} `json:"response"`
	ClusterPods struct {
		Value uint32 `json:"value"`
	} `json:"cluster_pods"`
	LocalPods struct {
		Value uint32 `json:"value"`
	} `json:"local_pods"`
	CPUMin struct {
		Value uint32 `json:"value"`
	} `json:"cpu_min"`
	CPUMax struct {
		Value uint32 `json:"value"`
	} `json:"cpu_max"`
	NodeName struct {
		Value string `json:"value"`
	} `json:"node_name"`
}

func (ay *AgentYang) SetNodeName(name string) {
	ay.NodeName.Value = name
}

func (ay *AgentYang) GetNodeName() string {
	return ay.NodeName.Value
}

func (ay *AgentYang) SetName(name string) {
	ay.Name.Value = name
}

func (ay *AgentYang) GetName() string {
	return ay.Name.Value
}

func (ay *AgentYang) SetResponse(response string) {
	ay.Response.Value = response
}

func (ay *AgentYang) GetResponse() string {
	return ay.Response.Value
}

func (ay *AgentYang) SetCPU(cpuMin uint32, cpuMax uint32) {
	ay.CPUMin.Value = cpuMin
	ay.CPUMax.Value = cpuMax
}

func (ay *AgentYang) GetCPU() (cpuMin uint32, cpuMax uint32) {
	return ay.CPUMin.Value, ay.CPUMax.Value
}

func (ay *AgentYang) SetPods(cluster uint32, local uint32) {
	ay.ClusterPods.Value = cluster
	ay.LocalPods.Value = local
}

func (ay *AgentYang) GetPods() (cluster uint32, local uint32) {
	return ay.ClusterPods.Value, ay.LocalPods.Value
}
