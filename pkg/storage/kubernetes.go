package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	srlinuxv1alpha1 "srlinux/k8/pkg/api/v1alpha1"

	"github.com/dougbtv/whereabouts/pkg/allocate"
	srlinuxtypes "github.com/dougbtv/whereabouts/pkg/types"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	log "k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// NewUnderlayPeer returns a new UnderlayPeer Client configured to a kubernetes CRD backend
func NewUnderlayPeer(containerID string, ipamConf srlinuxtypes.IPAMConfig) (*UnderlayPeerClient, error) {
	scheme := runtime.NewScheme()
	_ = srlinuxv1alpha1.AddToScheme(scheme)

	overrides := &clientcmd.ConfigOverrides{}
	if apiURL := ipamConf.Kubernetes.K8sAPIRoot; apiURL != "" {
		overrides.ClusterInfo = clientcmdapi.Cluster{
			Server: apiURL,
		}
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: ipamConf.Kubernetes.KubeConfigPath},
		overrides).ClientConfig()
	if err != nil {
		return nil, err
	}

	var namespace string
	if cfg, err := clientcmd.LoadFromFile(ipamConf.Kubernetes.KubeConfigPath); err != nil {
		return nil, err
	} else if ctx, ok := cfg.Contexts[cfg.CurrentContext]; ok && ctx != nil {
		namespace = ctx.Namespace
	} else {
		return nil, fmt.Errorf("k8s config: namespace not present in context")
	}
	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return nil, err
	}
	c, err := client.New(config, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}
	return &UnderlayPeerClient{c, ipamConf, containerID, namespace, DatastoreRetries}, nil
}

// KubernetesIPAM manages ip blocks in an kubernetes CRD backend
type UnderlayPeerClient struct {
	client      client.Client
	config      srlinuxtypes.IPAMConfig
	containerID string
	namespace   string
	retries     int
}

func toIPReservationList(allocations map[string]srlinuxv1alpha1.IPAllocation, firstip net.IP) []srlinuxtypes.IPReservation {
	reservelist := []srlinuxtypes.IPReservation{}
	for offset, a := range allocations {
		numOffset, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			// allocations that are invalid int64s should be ignored
			// toAllocationMap should be the only writer of offsets, via `fmt.Sprintf("%d", ...)``
			log.Errorf("Error decoding ip offset (backend: kubernetes): %v", err)
			continue
		}
		ip := allocate.IPAddOffset(firstip, uint64(numOffset))
		reservelist = append(reservelist, srlinuxtypes.IPReservation{IP: ip, ContainerID: a.ContainerID, PodRef: a.PodRef})
	}
	return reservelist
}

func toAllocationMap(reservelist []srlinuxtypes.IPReservation, firstip net.IP) map[string]srlinuxv1alpha1.IPAllocation {
	allocations := make(map[string]srlinuxv1alpha1.IPAllocation)
	for _, r := range reservelist {
		index := allocate.IPGetOffset(r.IP, firstip)
		allocations[fmt.Sprintf("%d", index)] = srlinuxv1alpha1.IPAllocation{ContainerID: r.ContainerID, PodRef: r.PodRef}
	}
	return allocations
}

// GetIPPool returns a storage.IPPool for the given range
func (i *UnderlayPeerClient) GetIPPool(ctx context.Context, ipRange string) (UnderlayPeer, error) {
	// v6 filter
	normalized := strings.ReplaceAll(ipRange, ":", "-")
	// replace subnet cidr slash
	normalized = strings.ReplaceAll(normalized, "/", "-")

	pool, err := i.getPeer(ctx, normalized, ipRange)
	if err != nil {
		return nil, err
	}

	firstIP, _, err := pool.ParseCIDR()
	if err != nil {
		return nil, err
	}

	return &KubernetesIPPool{i.client, i.containerID, firstIP, pool}, nil
}

func (i *UnderlayPeerClient) getPeer(ctx context.Context, name string, iprange string) (*srlinuxv1alpha1.UnderlayPeer, error) {
	peer := &srlinuxv1alpha1.UnderlayPeer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: i.namespace},
	}
	if err := i.client.Get(ctx, types.NamespacedName{Name: name, Namespace: i.namespace}, peer); errors.IsNotFound(err) {
		// pool does not exist, create it
		peer.ObjectMeta.Name = name
		peer.Spec.Range = iprange
		peer.Spec.Allocations = make(map[string]srlinuxv1alpha1.IPAllocation)
		if err := i.client.Create(ctx, peer); errors.IsAlreadyExists(err) {
			// the peer was just created -- allow retry
			return nil, &temporaryError{err}
		} else if err != nil {
			return nil, fmt.Errorf("k8s create error: %s", err)
		}
		// if the pool was created for the first time, trigger another retry of the allocation loop
		// so all of the metadata / resourceVersions are populated as necessary by the `client.Get` call
		return nil, &temporaryError{fmt.Errorf("k8s pool initialized")}
	} else if err != nil {
		return nil, fmt.Errorf("k8s get error: %s", err)
	}
	return peer, nil
}

// Status tests connectivity to the kubernetes backend
func (i *UnderlayPeerClient) Status(ctx context.Context) error {
	list := &srlinuxv1alpha1.UnderlayPeerList{}
	err := i.client.List(ctx, list, &client.ListOptions{Namespace: i.namespace})
	return err
}

// Close partially implements the Store interface
func (i *UnderlayPeerClient) Close() error {
	return nil
}

// KubernetesIPPool represents an IPPool resource and its parsed set of allocations
type KubernetesIPPool struct {
	client      client.Client
	containerID string
	firstIP     net.IP
	pool        *srlinuxv1alpha1.IPPool
}

// Allocations returns the initially retrieved set of allocations for this pool
func (p *KubernetesIPPool) Allocations() []srlinuxtypes.IPReservation {
	return toIPReservationList(p.pool.Spec.Allocations, p.firstIP)
}

// Update sets the pool allocated IP list to the given IP reservations
func (p *KubernetesIPPool) Update(ctx context.Context, reservations []srlinuxtypes.IPReservation) error {
	// marshal the current pool to serve as the base for the patch creation
	orig := p.pool.DeepCopy()
	origBytes, err := json.Marshal(orig)
	if err != nil {
		return err
	}

	// update the pool before marshalling once again
	p.pool.Spec.Allocations = toAllocationMap(reservations, p.firstIP)
	modBytes, err := json.Marshal(p.pool)
	if err != nil {
		return err
	}

	// create the patch
	patch, err := jsonpatch.CreatePatch(origBytes, modBytes)
	if err != nil {
		return err
	}

	// add additional tests to the patch
	ops := []jsonpatch.Operation{
		// ensure patch is applied to appropriate resource version only
		{Operation: "test", Path: "/metadata/resourceVersion", Value: orig.ObjectMeta.ResourceVersion},
	}
	for _, o := range patch {
		// safeguard add ops -- "add" will update existing paths, this "test" ensures the path is empty
		if o.Operation == "add" {
			var m map[string]interface{}
			ops = append(ops, jsonpatch.Operation{Operation: "test", Path: o.Path, Value: m})
		}
	}
	ops = append(ops, patch...)
	patchData, err := json.Marshal(ops)
	if err != nil {
		return err
	}

	// apply the patch
	err = p.client.Patch(ctx, orig, client.ConstantPatch(types.JSONPatchType, patchData))
	if err != nil {
		if errors.IsInvalid(err) {
			// expect "invalid" errors if any of the jsonpatch "test" Operations fail
			return &temporaryError{err}
		}
		return err
	}

	return nil
}

type temporaryError struct {
	error
}

func (t *temporaryError) Temporary() bool {
	return true
}

type temporary interface {
	Temporary() bool
}
