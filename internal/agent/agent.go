package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	log "k8s.io/klog"

	"github.com/brwallis/srlinux-go/pkg/ndk/nokia.com/srlinux/sdk/protos"
	"github.com/brwallis/srlinux-kbutler/internal/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// CfgTranxEntry contains an NDK mgr operation
type CfgTranxEntry struct {
	Op   protos.SdkMgrOperation
	Key  *protos.ConfigKey
	Data *string
}

type ServiceKey struct {
	Name      string
	Namespace string
}

type EndpointKey struct {
	ExternalAddress string
	Hostname        string
}

// Agent represents an instance of an NDK agent
type Agent struct {
	m *sync.RWMutex

	Name     string
	OwnAppID uint32
	StreamID uint64
	Client   protos.SdkMgrServiceClient
	GrpcConn *grpc.ClientConn
	Wg       sync.WaitGroup

	CfgTranxMap map[string][]CfgTranxEntry

	Yang         config.AgentYang
	YangService  map[ServiceKey]*config.Service
	YangEndpoint map[EndpointKey]*config.Endpoint
	YangRoot     string

	ServiceMap map[ServiceKey][]EndpointKey
}

func (a *Agent) GetName() string {
	return a.Name
}

func (a *Agent) GetGRPCConn() *grpc.ClientConn {
	return a.GrpcConn
}

func (a *Agent) GetStreamID() uint64 {
	return a.StreamID
}

func (a *Agent) SetStreamID(streamID uint64) {
	a.StreamID = streamID
}

func (a *Agent) UpdateTelemetry(jsPath *string, jsData *string) *protos.TelemetryUpdateResponse {
	ctx := context.Background()

	// Set up agent name
	ctx = metadata.AppendToOutgoingContext(ctx, "agent_name", a.Name)
	telClient := protos.NewSdkMgrTelemetryServiceClient(a.GrpcConn)

	key := &protos.TelemetryKey{JsPath: *jsPath}
	data := &protos.TelemetryData{JsonContent: *jsData}
	entry := &protos.TelemetryInfo{Key: key, Data: data}
	telReq := &protos.TelemetryUpdateRequest{}
	telReq.State = make([]*protos.TelemetryInfo, 0)
	telReq.State = append(telReq.State, entry)

	result, err := telClient.TelemetryAddOrUpdate(ctx, telReq)
	if err != nil {
		log.Fatalf("Could not update telemetry for key : %s", jsPath)
	}
	log.Infof("Telemetry add/update status: %s error_string: %s", result.GetStatus(), result.GetErrorStr())
	return result
}

func (a *Agent) UpdateEndpointTelemetry(serviceKey ServiceKey, endpointKey EndpointKey) {
	jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}.external_address{.address==\"%s\"&&.hostname==\"%s\"}", a.YangRoot, serviceKey.Name, serviceKey.Namespace, endpointKey.ExternalAddress, endpointKey.Hostname)
	jsData, err := json.Marshal(a.YangEndpoint[endpointKey])
	if err != nil {
		log.Fatalf("Can not marshal config data: error %s", err)
	}
	jsString := string(jsData)
	result := a.UpdateTelemetry(&jsPath, &jsString)

	log.Infof("Telemetry add/update status: %s error_string: %s", result.GetStatus(), result.GetErrorStr())
}

func (a *Agent) UpdateServiceTelemetry(serviceKey ServiceKey) {
	jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}", a.YangRoot, serviceKey.Name, serviceKey.Namespace)
	jsData, err := json.Marshal(a.YangService[serviceKey])
	if err != nil {
		log.Fatalf("Can not marshal config data: error %s", err)
	}
	jsString := string(jsData)
	result := a.UpdateTelemetry(&jsPath, &jsString)

	log.Infof("Telemetry add/update status: %s error_string: %s", result.GetStatus(), result.GetErrorStr())
}

func (a *Agent) UpdateBaseTelemetry() {
	jsData, err := json.Marshal(a.Yang)
	if err != nil {
		log.Fatalf("Can not marshal config data: error %s", err)
	}
	jsString := string(jsData)
	result := a.UpdateTelemetry(&a.YangRoot, &jsString)

	log.Infof("Telemetry add/update status: %s error_string: %s", result.GetStatus(), result.GetErrorStr())
}

// DeleteEndpoint sends a delete to NDK for the specified service + endpoint
func (a *Agent) DeleteEndpoint(serviceKey ServiceKey, endpointKey EndpointKey) {
	jsPath := fmt.Sprintf("%s.service{.service_name==\"%s\"&&.namespace==\"%s\"}.external_address{.address==\"%s\"&&.hostname==\"%s\"}", a.YangRoot, serviceKey.Name, serviceKey.Namespace, endpointKey.ExternalAddress, endpointKey.Hostname)
	a.DeleteTelemetry(&jsPath)
}

// DeleteTelemetry sends a delete to NDK for the specified path
func (a *Agent) DeleteTelemetry(JsPath *string) {
	ctx := context.Background()

	// Set up agent name
	ctx = metadata.AppendToOutgoingContext(ctx, "agent_name", a.Name)
	telClient := protos.NewSdkMgrTelemetryServiceClient(a.GrpcConn)

	key := &protos.TelemetryKey{JsPath: *JsPath}
	telReq := &protos.TelemetryDeleteRequest{}
	telReq.Key = make([]*protos.TelemetryKey, 0)
	telReq.Key = append(telReq.Key, key)

	r1, err := telClient.TelemetryDelete(ctx, telReq)
	if err != nil {
		log.Fatalf("Could not delete telemetry for key : %s", *JsPath)
	}
	log.Infof("Telemetry delete status: %s error_string: %s", r1.GetStatus(), r1.GetErrorStr())
}

// AgentInit initializes an agent
func (a *Agent) Init(name string, ndkAddress string, yangRoot string) {
	var err error

	// Set up a connection to the server.
	conn, err := grpc.Dial(ndkAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	// Set up base service client
	client := protos.NewSdkMgrServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set up agent name
	ctx = metadata.AppendToOutgoingContext(ctx, "agent_name", name)

	// Register agent with NDK manager
	r, err := client.AgentRegister(ctx, &protos.AgentRegistrationRequest{})
	if err != nil {
		log.Fatalf("Could not register: %v", err)
	}
	log.Infof("Agent registration status: %s AppId: %d\n", r.Status, r.GetAppId())

	a.Name = name
	a.GrpcConn = conn
	a.Client = client
	a.OwnAppID = r.GetAppId()
	a.YangRoot = yangRoot

	a.CfgTranxMap = make(map[string][]CfgTranxEntry)
	a.YangService = make(map[ServiceKey]*config.Service)
	a.YangEndpoint = make(map[EndpointKey]*config.Endpoint)
	a.ServiceMap = make(map[ServiceKey][]EndpointKey)

	subscribeStreams(a)
}

// SubscribeStreams subscribes for config notifications
func subscribeStreams(a *Agent) {
	ctx := context.Background()
	// Set up agent name
	ctx = metadata.AppendToOutgoingContext(ctx, "agent_name", a.GetName())

	notifRegReq := &protos.NotificationRegisterRequest{Op: protos.NotificationRegisterRequest_Create}
	r3, err := a.Client.NotificationRegister(ctx, notifRegReq)
	if err != nil {
		log.Fatalf("Could not register for notification : %v", err)
	}
	log.Infof("Notification registration status : %s stream_id %v\n", r3.Status, r3.GetStreamId())

	a.SetStreamID(r3.GetStreamId())
	// agent.StreamID = r3.GetStreamId()

	cfgEntry := &protos.NotificationRegisterRequest_Config{Config: &protos.ConfigSubscriptionRequest{}}
	cfgReq := &protos.NotificationRegisterRequest{
		Op:                protos.NotificationRegisterRequest_AddSubscription,
		StreamId:          r3.GetStreamId(),
		SubscriptionTypes: cfgEntry,
	}
	r4, err := a.Client.NotificationRegister(ctx, cfgReq)
	if err != nil {
		log.Fatalf("Could not register for config notification : %v", err)
	}
	log.Infof("Config notification registration status : %s stream_id %v\n", r4.Status, r4.GetStreamId())
}

// RunRecvNotification is called when a notification is received
func (a *Agent) ReceiveNotifications() {
	defer a.Wg.Done()

	ctx := context.Background()

	// Set up agent name
	ctx = metadata.AppendToOutgoingContext(ctx, "agent_name", a.Name)
	notifClient := protos.NewSdkNotificationServiceClient(a.GrpcConn)
	subReq := &protos.NotificationStreamRequest{StreamId: a.StreamID}
	stream, err := notifClient.NotificationStream(ctx, subReq)
	if err != nil {
		log.Fatalf("Could not subscribe for notification : %v", err)
	}

	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive a notification : %v", err)
			}
			HandleNotificationEvent(in, a)
		}
	}()
	<-waitc
}

// HandleKButlerConfigEvent handles configuration events for the .kbutler node
func HandleKButlerConfigEvent(op protos.SdkMgrOperation, key *protos.ConfigKey, data *string, a *Agent) {
	log.Infof("\n jspath %s keys %v", key.GetJsPath(), key.GetKeys())

	if data != nil {
		log.Infof("\n data %v", *data)
	}

	if data == nil {
		log.Infof("\nNo data found")
		if op == protos.SdkMgrOperation_Delete {
			log.Infof("\nDelete operation")
			a.DeleteTelemetry(&a.YangRoot)
		}
		return
	}

	cur := config.AgentYang{}
	// cur := &yang.Device{}
	if err := json.Unmarshal([]byte(*data), &cur); err != nil {
		log.Fatalf("Can not unmarshal config data: %s error %s", *data, err)
	}

	log.Infof("\nkey %v", *key)
	log.Infof("\nkey %v doing something now", *key)
}

// HandleConfigEvent handles a configuration event, calling the correct function to handle it
func HandleConfigEvent(op protos.SdkMgrOperation, key *protos.ConfigKey, data *string, a *Agent) {
	log.Infof("\nkey %v", *key)

	if key.GetJsPath() != ".commit.end" {
		a.CfgTranxMap[key.GetJsPath()] = append(a.CfgTranxMap[key.GetJsPath()], CfgTranxEntry{Op: op, Key: key, Data: data})
		return
	}

	for _, item := range a.CfgTranxMap[".kbutler.config-node"] {
		log.Infof("%s", item)
		// HandleKButlerConfigEvent(item.Op, item.Key, item.Data)
	}

	for _, item := range a.CfgTranxMap[a.YangRoot] {
		HandleKButlerConfigEvent(item.Op, item.Key, item.Data, a)
	}

	// Delete all current candidate list.
	a.CfgTranxMap = make(map[string][]CfgTranxEntry)
}

// HandleNotificationEvent handles a notification event from NDK
func HandleNotificationEvent(in *protos.NotificationStreamResponse, a *Agent) {
	for _, item := range in.Notification {
		switch x := item.SubscriptionTypes.(type) {
		case *protos.Notification_Config:
			resp := item.GetConfig()
			if resp.Data != nil {
				HandleConfigEvent(resp.Op, resp.Key, &resp.Data.Json, a)
			} else {
				HandleConfigEvent(resp.Op, resp.Key, nil, a)
			}
		default:
			log.Infof("\nGot unhandled message %s ", x)
		}
	}
}
