package storage

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dougbtv/whereabouts/pkg/allocate"
	"github.com/dougbtv/whereabouts/pkg/types"
	log "k8s.io/klog"
)

var (
	// RequestTimeout defines how long the context timesout in
	RequestTimeout = 10 * time.Second

	// DatastoreRetries defines how many retries are attempted when updating/creating the UnderlayPeer
	DatastoreRetries = 100
)

// UnderlayPeer is the interface that represents an instance of an underlay peer
type UnderlayPeer interface {
	Allocations() []types.IPReservation
	Update(ctx context.Context, reservations []types.IPReservation) error
}

// UnderlayPeerMgr is the interface that wraps the basic under peer management methods on the underlying storage backend
type UnderlayPeerMgr interface {
	GetPeer(ctx context.Context, peerIP string) (UnderlayPeer, error)
	Status(ctx context.Context) error
	Close() error
}

// UnderlayPeerManagement manages underlay peer creation and deletion
func UnderlayPeerManagement(mode int, ipamConf types.IPAMConfig, containerID string, podRef string) (net.IPNet, error) {
	var uPeerMgr UnderlayPeerMgr
	var uPeer UnderlayPeer
	var newip net.IPNet
	var err error

	log.Infof("UnderlayPeerManagement -- mode: %v / host: %v / containerID: %v / podRef: %v", mode, ipamConf.EtcdHost, containerID, podRef)

	// Skip invalid modes
	// switch mode {
	// case types.Allocate, types.Deallocate:
	// default:
	// 	return newip, fmt.Errorf("Got an unknown mode passed to IPManagement: %v", mode)
	// }

	// Call
	uPeerClient, err = NewUnderlayPeer(containerID, ipamConf)
	if err != nil {
		log.Errorf("underlayPeerClientMgr client initialization error: %v", err)
		return newip, fmt.Errorf("IPAM %s client initialization error: %v", ipamConf.Datastore, err)
	}
	defer uPeerMgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	// Check our connectivity first
	if err := uPeerMgr.Status(ctx); err != nil {
		log.Errorf("IPAM connectivity error: %v", err)
		return newip, err
	}

	// handle the ip add/del until successful
RETRYLOOP:
	for j := 0; j < DatastoreRetries; j++ {
		select {
		case <-ctx.Done():
			return newip, nil
		default:
			// retry the IPAM loop if the context has not been cancelled
		}

		peer, err = uPeerMgr.GetUnderlayPeer(ctx, ipamConf.Range)
		if err != nil {
			log.Errorf("IPAM error reading pool allocations (attempt: %d): %v", j, err)
			if e, ok := err.(temporary); ok && e.Temporary() {
				continue
			}
			return newip, err
		}

		reservelist := pool.Allocations()
		var updatedreservelist []types.IPReservation
		switch mode {
		case types.Allocate:
			newip, updatedreservelist, err = allocate.AssignIP(ipamConf, reservelist, containerID, podRef)
			if err != nil {
				log.Errorf("Error assigning IP: %v", err)
				return newip, err
			}
		case types.Deallocate:
			updatedreservelist, err = allocate.DeallocateIP(ipamConf.Range, reservelist, containerID)
			if err != nil {
				log.Errorf("Error deallocating IP: %v", err)
				return newip, err
			}
		}

		err = peer.Update(ctx, updatedreservelist)
		if err != nil {
			log.Errorf("IPAM error updating pool (attempt: %d): %v", j, err)
			if e, ok := err.(temporary); ok && e.Temporary() {
				continue
			}
			break RETRYLOOP
		}
		break RETRYLOOP
	}

	return newip, err
}
