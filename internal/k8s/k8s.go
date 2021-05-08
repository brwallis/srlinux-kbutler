package k8s

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8Init initializes the interface towards the K8 API server
func K8Init() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	ClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return ClientSet
}

// Client initializes the interface towards the K8 API server
func Client(kubeConfig string) *kubernetes.Clientset {
	var config *rest.Config
	var clientSet *kubernetes.Clientset
	var err error
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" && os.Getenv("KUBERNETES_SERVICE_PORT") != "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		clientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
	} else {
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			panic(err.Error())
		}
		clientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
	}
	return clientSet
}

// countPods takes a K8 clientSet and a node name and returns a count of pods matching
func countPods(clientSet *kubernetes.Clientset, nodeName string) uint32 {
	var fieldSelector string
	if len(nodeName) > 0 {
		fieldSelector = "spec.nodeName=" + nodeName
	}
	pods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		panic(err.Error())
	}
	return uint32(len(pods.Items))
}

// PodCounter well... counts pods!
func PodCounter(clientSet *kubernetes.Clientset, nodeName string) (totalPodsCluster uint32, totalPodsLocal uint32) {
	// get pods in all nodes by omitting node
	totalPodsCluster = countPods(clientSet, "")
	// Or specify to get pods on a particular node
	totalPodsLocal = countPods(clientSet, nodeName)
	fmt.Printf("There are %d pods in the cluster\n", totalPodsCluster)
	fmt.Printf("There are %d pods on node %s\n", totalPodsLocal, nodeName)

	return totalPodsCluster, totalPodsLocal
	// Examples for error handling:
	// - Use helper functions e.g. errors.IsNotFound()
	// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
	//_, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
	//if errors.IsNotFound(err) {
	//	fmt.Printf("Pod example-xxxxx not found in default namespace\n")
	//} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
	//	fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
	//} else if err != nil {
	//	panic(err.Error())
	//} else {
	//	fmt.Printf("Found example-xxxxx pod in default namespace\n")
	//}

}
