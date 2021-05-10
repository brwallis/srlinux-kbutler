package main

import (
	"time"

	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog"

	"github.com/brwallis/srlinux-kbutler/internal/agent"
	"github.com/brwallis/srlinux-kbutler/internal/endpointmgr"
	"github.com/brwallis/srlinux-kbutler/internal/k8s"
	"github.com/brwallis/srlinux-kbutler/internal/servicemgr"
)

const (
	ndkAddress = "localhost:50053"
	agentName  = "kbutler"
	yangRoot   = ".kbutler"
)

// Global vars
var (
	KButler agent.Agent
)

// SetName publishes the baremetal's hostname into the container
func SetName(nodeName string) {
	log.Infof("Setting node name...")
	// KButler.Yang.NodeName = &nodeName
	// KButler.Yang.SetNodeName(nodeName)
	//KButler.Yang.NodeName.Value = nodeName
	// JsData, err := json.Marshal(KButler.Yang)
	// if err != nil {
	// 	log.Fatalf("Can not marshal config data: error %s", err)
	// }
	// JsString := string(JsData)
	// log.Infof("JsPath: %s", KButler.YangRoot)
	// log.Infof("JsString: %s", JsString)
	KButler.UpdateTelemetry()
	// ndk.UpdateTelemetry(KButler, &JsPath, &JsString)
}

// PodCounterMgr counts pods!
func PodCounterMgr(clientSet *kubernetes.Clientset, nodeName string) {
	for {
		// totalPodsCluster, totalPodsLocal := k8s.PodCounter(clientSet, nodeName)
		// KButler.Yang.SetPods(totalPodsCluster, totalPodsLocal)
		KButler.UpdateTelemetry()
		time.Sleep(5 * time.Second)
	}
}

func main() {
	// var err error
	// nodeName := os.Getenv("KUBERNETES_NODE_NAME")
	// nodeIP := os.Getenv("KUBERNETES_NODE_IP")

	log.Infof("Initializing NDK...")
	KButler = agent.Agent{}
	KButler.Init(agentName, ndkAddress, yangRoot)

	log.Infof("Starting to receive notifications from NDK...")
	KButler.Wg.Add(1)
	go KButler.ReceiveNotifications()

	time.Sleep(2 * time.Second)
	log.Infof("Initializing K8 client...")
	KubeClientSet := k8s.K8Init()

	// log.Infof("Starting PodCounterMgr...")
	// KButler.Wg.Add(1)
	// go PodCounterMgr(KubeClientSet, nodeName)

	log.Infof("Starting ServiceMgr...")
	KButler.Wg.Add(1)
	go servicemgr.ServiceMgr(KubeClientSet, &KButler)

	log.Infof("Starting EndpointMgr...")
	KButler.Wg.Add(1)
	go endpointmgr.EndpointMgr(KubeClientSet, &KButler)

	KButler.Wg.Wait()

	KButler.GrpcConn.Close()
}
