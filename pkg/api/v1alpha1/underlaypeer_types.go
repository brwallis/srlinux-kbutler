// package v1alpha1

// import (
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// // UnderlayPeerSpec defines the desired state of UnderlayPeer
// type UnderlayPeerSpec struct {
// 	Node     string `json:"node,omitempty"` // If defined, gives this underlaypeer node scope, otherwise scope is global
// 	PeerIP   string `json:"peerIP"`
// 	asNumber int    `json:"asNumber"`
// }

// // UnderlayPeer is the Schema for the underlaypeers API
// type UnderlayPeer struct {
// 	metav1.TypeMeta   `json:",inline"`
// 	metav1.ObjectMeta `json:"metadata,omitempty"`

// 	Spec UnderlayPeerSpec `json:"spec,omitempty"`
// }

// // +kubebuilder:object:root=true

// // UnderlayPeerList contains a list of UnderlayPeers
// type UnderlayPeerList struct {
// 	metav1.TypeMeta `json:",inline"`
// 	metav1.ListMeta `json:"metadata,omitempty"`
// 	Items           []UnderlayPeer `json:"items"`
// }

// func init() {
// 	SchemeBuilder.Register(&UnderlayPeer{}, &UnderlayPeerList{})
// }
