package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog"

	// "srlinux.io/kbutler/pkg/storage"
	"github.com/brwallis/srlinux-kbutler/internal/agent"
	"github.com/brwallis/srlinux-kbutler/internal/cpumgr"
	"github.com/brwallis/srlinux-kbutler/internal/k8s"
	"github.com/brwallis/srlinux-kbutler/internal/netmgr"
	"github.com/vishvananda/netns"
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
	KButler.Yang.SetNodeName(nodeName)
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
		totalPodsCluster, totalPodsLocal := k8s.PodCounter(clientSet, nodeName)
		KButler.Yang.SetPods(totalPodsCluster, totalPodsLocal)
		KButler.UpdateTelemetry()
		time.Sleep(5 * time.Second)
	}
}

func main() {
	var err error
	nodeName := os.Getenv("KUBERNETES_NODE_NAME")
	nodeIP := os.Getenv("KUBERNETES_NODE_IP")

	srlNS, err := netns.Get()
	log.Infof("Running in namespace: %#v", srlNS)
	toWrite := []byte(fmt.Sprint(srlNS))
	err = ioutil.WriteFile("/etc/opt/srlinux/namespace", toWrite, 0644)
	if err != nil {
		log.Fatalf("Unable to write namespace: %v to file /etc/opt/srlinux/namespace")
	}

	log.Infof("Initializing NDK...")
	KButler = agent.Agent{}
	KButler.Init(agentName, ndkAddress, yangRoot)

	log.Infof("Starting to receive notifications from NDK...")
	KButler.Wg.Add(1)
	go KButler.ReceiveNotifications()

	time.Sleep(2 * time.Second)
	log.Infof("Initializing K8 client...")
	KubeClientSet := k8s.K8Init()

	log.Infof("Starting PodCounterMgr...")
	KButler.Wg.Add(1)
	go PodCounterMgr(KubeClientSet, nodeName)

	log.Infof("Starting CPUMgr...")
	KButler.Wg.Add(1)
	go cpumgr.CPUMgr(KubeClientSet, &KButler)

	time.Sleep(2 * time.Second)
	log.Infof("Starting NetMgr...")
	KButler.Wg.Add(1)
	go netmgr.NetMgr(KubeClientSet, nodeIP, &KButler)

	time.Sleep(2 * time.Second)
	log.Infof("Starting NameMgr...")
	KButler.Wg.Add(1)
	go SetName(nodeName)

	KButler.Wg.Wait()

	KButler.GrpcConn.Close()
}
