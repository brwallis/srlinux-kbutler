package main

import (
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	log "k8s.io/klog"

	"github.com/brwallis/srlinux-kbutler/internal/agent"
	"github.com/brwallis/srlinux-kbutler/internal/endpointmgr"
	"github.com/brwallis/srlinux-kbutler/internal/k8s"
	"github.com/brwallis/srlinux-kbutler/internal/servicemgr"
)

const (
	// ndkAddress = "localhost:50053"
	ndkAddress = "unix:///opt/srlinux/var/run/sr_sdk_service_manager:50053"
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
	KButler.UpdateBaseTelemetry()
	// ndk.UpdateTelemetry(KButler, &JsPath, &JsString)
}

// PodCounterMgr counts pods!
func PodCounterMgr(clientSet *kubernetes.Clientset, nodeName string) {
	for {
		// totalPodsCluster, totalPodsLocal := k8s.PodCounter(clientSet, nodeName)
		// KButler.Yang.SetPods(totalPodsCluster, totalPodsLocal)
		KButler.UpdateBaseTelemetry()
		time.Sleep(5 * time.Second)
	}
}

// SetController updates the NDK with a Kubernetes controller IP address
func SetController(kubeConfig *rest.Config) {
	if kubeConfig.Host != "" {
		KButler.Yang.Controller.Value = kubeConfig.Host
	} else {
		log.Infof("Unable to parse Kubernetes API server from kubeconfig")
		KButler.Yang.Controller.Value = "unknown"
	}
	KButler.UpdateBaseTelemetry()
}

func main() {
	var KubeClientSet *kubernetes.Clientset
	var KubeConfig *rest.Config
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
	// KubeClientSet := k8s.K8Init()
	if os.Getenv("KUBERNETES_CONFIG") != "" {
		KubeClientSet, KubeConfig = k8s.Client(os.Getenv("KUBERNETES_CONFIG"))
	} else {
		log.Errorf("Unable to initialize K8 client set, env var KUBERNETES_CONFIG not set")
	}
	log.Infof("KubeConfig: %#v", KubeConfig)
	SetController(KubeConfig)

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
