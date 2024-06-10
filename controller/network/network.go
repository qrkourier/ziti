/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package network

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/openziti/foundation/v2/concurrenz"
	"github.com/openziti/foundation/v2/goroutines"
	"github.com/openziti/storage/objectz"
	fabricMetrics "github.com/openziti/ziti/common/metrics"
	"github.com/openziti/ziti/common/pb/mgmt_pb"
	"github.com/openziti/ziti/controller/event"
	"github.com/openziti/ziti/controller/raft"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openziti/foundation/v2/versions"
	"github.com/openziti/ziti/controller/command"

	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/channel/v2/protobufs"
	"github.com/openziti/foundation/v2/debugz"
	"github.com/openziti/foundation/v2/errorz"
	"github.com/openziti/foundation/v2/sequence"
	"github.com/openziti/identity"
	"github.com/openziti/metrics"
	"github.com/openziti/metrics/metrics_pb"
	"github.com/openziti/storage/boltz"
	"github.com/openziti/ziti/common/ctrl_msg"
	"github.com/openziti/ziti/common/logcontext"
	"github.com/openziti/ziti/common/pb/ctrl_pb"
	"github.com/openziti/ziti/common/trace"
	"github.com/openziti/ziti/controller/db"
	"github.com/openziti/ziti/controller/xt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
)

const SmartRerouteAttempt = 99969996

// Config provides the values needed to create a Network instance
type Config interface {
	GetId() *identity.TokenId
	GetMetricsRegistry() metrics.Registry
	GetOptions() *Options
	GetCommandDispatcher() command.Dispatcher
	GetDb() boltz.Db
	GetVersionProvider() versions.VersionProvider
	GetEventDispatcher() event.Dispatcher
	GetCloseNotify() <-chan struct{}
}

type InspectTarget func(string) (bool, *string, error)

type Network struct {
	*Managers
	nodeId                 string
	options                *Options
	assembleAndCleanC      chan struct{}
	linkController         *linkController
	forwardingFaults       chan struct{}
	circuitController      *circuitController
	routeSenderController  *routeSenderController
	sequence               *sequence.Sequence
	eventDispatcher        event.Dispatcher
	traceController        trace.Controller
	routerPresenceHandlers []RouterPresenceHandler
	capabilities           []string
	closeNotify            <-chan struct{}
	watchdogCh             chan struct{}
	lock                   sync.Mutex
	strategyRegistry       xt.Registry
	lastSnapshot           time.Time
	metricsRegistry        metrics.Registry
	VersionProvider        versions.VersionProvider

	serviceEventMetrics          metrics.UsageRegistry
	serviceDialSuccessCounter    metrics.IntervalCounter
	serviceDialFailCounter       metrics.IntervalCounter
	serviceDialTimeoutCounter    metrics.IntervalCounter
	serviceDialOtherErrorCounter metrics.IntervalCounter

	serviceTerminatorTimeoutCounter           metrics.IntervalCounter
	serviceTerminatorConnectionRefusedCounter metrics.IntervalCounter
	serviceInvalidTerminatorCounter           metrics.IntervalCounter
	serviceMisconfiguredTerminatorCounter     metrics.IntervalCounter

	config Config

	inspectionTargets concurrenz.CopyOnWriteSlice[InspectTarget]
}

func NewNetwork(config Config) (*Network, error) {
	stores, err := db.InitStores(config.GetDb(), config.GetCommandDispatcher().GetRateLimiter())
	if err != nil {
		return nil, err
	}

	if config.GetOptions().IntervalAgeThreshold != 0 {
		metrics.SetIntervalAgeThreshold(config.GetOptions().IntervalAgeThreshold)
		logrus.Infof("set interval age threshold to '%v'", config.GetOptions().IntervalAgeThreshold)
	}
	serviceEventMetrics := metrics.NewUsageRegistry(config.GetId().Token, nil, config.GetCloseNotify())

	network := &Network{
		nodeId:                config.GetId().Token,
		options:               config.GetOptions(),
		assembleAndCleanC:     make(chan struct{}, 1),
		linkController:        newLinkController(config.GetOptions()),
		forwardingFaults:      make(chan struct{}, 1),
		circuitController:     newCircuitController(),
		routeSenderController: newRouteSenderController(),
		sequence:              sequence.NewSequence(),
		eventDispatcher:       config.GetEventDispatcher(),
		traceController:       trace.NewController(config.GetCloseNotify()),
		closeNotify:           config.GetCloseNotify(),
		watchdogCh:            make(chan struct{}, 1),
		strategyRegistry:      xt.GlobalRegistry(),
		lastSnapshot:          time.Now().Add(-time.Hour),
		metricsRegistry:       config.GetMetricsRegistry(),
		VersionProvider:       config.GetVersionProvider(),

		serviceEventMetrics:          serviceEventMetrics,
		serviceDialSuccessCounter:    serviceEventMetrics.IntervalCounter("service.dial.success", time.Minute),
		serviceDialFailCounter:       serviceEventMetrics.IntervalCounter("service.dial.fail", time.Minute),
		serviceDialTimeoutCounter:    serviceEventMetrics.IntervalCounter("service.dial.timeout", time.Minute),
		serviceDialOtherErrorCounter: serviceEventMetrics.IntervalCounter("service.dial.error_other", time.Minute),

		serviceTerminatorTimeoutCounter:           serviceEventMetrics.IntervalCounter("service.dial.terminator.timeout", time.Minute),
		serviceTerminatorConnectionRefusedCounter: serviceEventMetrics.IntervalCounter("service.dial.terminator.connection_refused", time.Minute),
		serviceInvalidTerminatorCounter:           serviceEventMetrics.IntervalCounter("service.dial.terminator.invalid", time.Minute),
		serviceMisconfiguredTerminatorCounter:     serviceEventMetrics.IntervalCounter("service.dial.terminator.misconfigured", time.Minute),

		config: config,
	}

	routerCommPool, err := network.createRouterCommPool(config)
	if err != nil {
		return nil, err
	}
	network.Managers = NewManagers(network, config.GetCommandDispatcher(), config.GetDb(), stores, routerCommPool)
	network.Managers.Inspections.network = network

	network.AddCapability("ziti.fabric")
	network.showOptions()
	network.relayControllerMetrics()
	network.AddRouterPresenceHandler(network.Managers.RouterMessaging)
	go network.Managers.RouterMessaging.run()

	return network, nil
}

func (network *Network) createRouterCommPool(config Config) (goroutines.Pool, error) {
	poolConfig := goroutines.PoolConfig{
		QueueSize:   config.GetOptions().RouterComm.QueueSize,
		MinWorkers:  0,
		MaxWorkers:  config.GetOptions().RouterComm.MaxWorkers,
		IdleTime:    30 * time.Second,
		CloseNotify: config.GetCloseNotify(),
		PanicHandler: func(err interface{}) {
			pfxlog.Logger().WithField(logrus.ErrorKey, err).WithField("backtrace", string(debug.Stack())).Error("panic during message send to router")
		},
	}

	fabricMetrics.ConfigureGoroutinesPoolMetrics(&poolConfig, config.GetMetricsRegistry(), "pool.router.messaging")

	pool, err := goroutines.NewPool(poolConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating router messaging pool")
	}
	return pool, nil
}

func (network *Network) relayControllerMetrics() {
	go func() {
		timer := time.NewTicker(network.options.MetricsReportInterval)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				if msg := network.metricsRegistry.Poll(); msg != nil {
					network.eventDispatcher.AcceptMetricsMsg(msg)
				}
			case <-network.closeNotify:
				return
			}
		}
	}()
}

func (network *Network) InitServiceCounterDispatch(handler metrics.Handler) {
	network.serviceEventMetrics.StartReporting(handler, network.GetOptions().MetricsReportInterval, 10)
}

func (network *Network) GetAppId() string {
	return network.nodeId
}

func (network *Network) GetOptions() *Options {
	return network.options
}

func (network *Network) GetDb() boltz.Db {
	return network.db
}

func (network *Network) GetStores() *db.Stores {
	return network.stores
}

func (network *Network) GetManagers() *Managers {
	return network.Managers
}

func (network *Network) GetConnectedRouter(routerId string) *Router {
	return network.Routers.getConnected(routerId)
}

func (network *Network) GetReloadedRouter(routerId string) (*Router, error) {
	network.Routers.RemoveFromCache(routerId)
	return network.Routers.Read(routerId)
}

func (network *Network) GetRouter(routerId string) (*Router, error) {
	return network.Routers.Read(routerId)
}

func (network *Network) AllConnectedRouters() []*Router {
	return network.Routers.allConnected()
}

func (network *Network) GetLink(linkId string) (*Link, bool) {
	return network.linkController.get(linkId)
}

func (network *Network) GetAllLinks() []*Link {
	return network.linkController.all()
}

func (network *Network) GetAllLinksForRouter(routerId string) []*Link {
	r := network.GetConnectedRouter(routerId)
	if r == nil {
		return nil
	}
	return r.routerLinks.GetLinks()
}

func (network *Network) GetCircuit(circuitId string) (*Circuit, bool) {
	return network.circuitController.get(circuitId)
}

func (network *Network) GetAllCircuits() []*Circuit {
	return network.circuitController.all()
}

func (network *Network) GetCircuitStore() *objectz.ObjectStore[*Circuit] {
	return network.circuitController.store
}

func (network *Network) GetLinkStore() *objectz.ObjectStore[*Link] {
	return network.linkController.store
}

func (network *Network) RouteResult(rs *RouteStatus) bool {
	return network.routeSenderController.forwardRouteResult(rs)
}

func (network *Network) newRouteSender(circuitId string) *routeSender {
	rs := newRouteSender(circuitId, network.options.RouteTimeout, network, network.Terminators)
	network.routeSenderController.addRouteSender(rs)
	return rs
}

func (network *Network) removeRouteSender(rs *routeSender) {
	network.routeSenderController.removeRouteSender(rs)
}

func (network *Network) GetEventDispatcher() event.Dispatcher {
	return network.eventDispatcher
}

func (network *Network) GetTraceController() trace.Controller {
	return network.traceController
}

func (network *Network) GetMetricsRegistry() metrics.Registry {
	return network.metricsRegistry
}

func (network *Network) GetServiceEventsMetricsRegistry() metrics.UsageRegistry {
	return network.serviceEventMetrics
}

func (network *Network) GetCloseNotify() <-chan struct{} {
	return network.closeNotify
}

func (network *Network) ConnectedRouter(id string) bool {
	return network.Routers.IsConnected(id)
}

func (network *Network) ConnectRouter(r *Router) {
	network.linkController.buildRouterLinks(r)
	network.Routers.markConnected(r)

	time.AfterFunc(250*time.Millisecond, network.notifyAssembleAndClean)

	for _, h := range network.routerPresenceHandlers {
		go h.RouterConnected(r)
	}
	go network.ValidateTerminators(r)
}

func (network *Network) ValidateTerminators(r *Router) {
	logger := pfxlog.Logger().WithField("routerId", r.Id)
	result, err := network.Terminators.Query(fmt.Sprintf(`router.id = "%v" limit none`, r.Id))
	if err != nil {
		logger.WithError(err).Error("failed to get terminators for router")
		return
	}

	logger.Debugf("%v terminators to validate", len(result.Entities))
	if len(result.Entities) == 0 {
		return
	}

	network.Managers.RouterMessaging.ValidateRouterTerminators(result.Entities)
}

type LinkValidationCallback func(detail *mgmt_pb.RouterLinkDetails)

func (n *Network) ValidateLinks(filter string, cb LinkValidationCallback) (int64, func(), error) {
	result, err := n.Routers.BaseList(filter)
	if err != nil {
		return 0, nil, err
	}

	sem := concurrenz.NewSemaphore(10)

	evalF := func() {
		for _, router := range result.Entities {
			connectedRouter := n.GetConnectedRouter(router.Id)
			if connectedRouter != nil {
				sem.Acquire()
				go func() {
					defer sem.Release()
					n.linkController.ValidateRouterLinks(n, connectedRouter, cb)
				}()
			} else {
				n.linkController.reportRouterLinksError(router, errors.New("router not connected"), cb)
			}
		}
	}

	return int64(len(result.Entities)), evalF, nil
}

type SdkTerminatorValidationCallback func(detail *mgmt_pb.RouterSdkTerminatorsDetails)

func (n *Network) ValidateRouterSdkTerminators(filter string, cb SdkTerminatorValidationCallback) (int64, func(), error) {
	result, err := n.Routers.BaseList(filter)
	if err != nil {
		return 0, nil, err
	}

	sem := concurrenz.NewSemaphore(10)

	evalF := func() {
		for _, router := range result.Entities {
			connectedRouter := n.GetConnectedRouter(router.Id)
			if connectedRouter != nil {
				sem.Acquire()
				go func() {
					defer sem.Release()
					n.Routers.ValidateRouterSdkTerminators(connectedRouter, cb)
				}()
			} else {
				n.Routers.reportRouterSdkTerminatorsError(router, errors.New("router not connected"), cb)
			}
		}
	}

	return int64(len(result.Entities)), evalF, nil
}

func (network *Network) DisconnectRouter(r *Router) {
	// 1: remove Links for Router
	for _, l := range r.routerLinks.GetLinks() {
		wasConnected := l.CurrentState().Mode == Connected
		if l.Src.Id == r.Id {
			network.linkController.remove(l)
		}
		if wasConnected {
			network.RerouteLink(l)
		}
	}
	// 2: remove Router
	network.Routers.markDisconnected(r)

	for _, h := range network.routerPresenceHandlers {
		h.RouterDisconnected(r)
	}

	network.notifyAssembleAndClean()
}

func (network *Network) notifyAssembleAndClean() {
	select {
	case network.assembleAndCleanC <- struct{}{}:
	default:
	}
}

func (network *Network) NotifyExistingLink(id string, iteration uint32, linkProtocol, dialAddress string, srcRouter *Router, dstRouterId string) {
	log := pfxlog.Logger().
		WithField("routerId", srcRouter.Id).
		WithField("linkId", id).
		WithField("destRouterId", dstRouterId).
		WithField("iteration", iteration)

	src := network.Routers.getConnected(srcRouter.Id)
	if src == nil {
		log.Info("ignoring links message processed after router disconnected")
		return
	}

	if src != srcRouter || !srcRouter.Connected.Load() {
		log.Info("ignoring links message processed from old router connection")
		return
	}

	dst := network.Routers.getConnected(dstRouterId)
	if dst == nil {
		network.NotifyLinkIdEvent(id, event.LinkFromRouterDisconnectedDest)
	}

	link, created := network.linkController.routerReportedLink(id, iteration, linkProtocol, dialAddress, srcRouter, dst, dstRouterId)
	if created {
		network.NotifyLinkEvent(link, event.LinkFromRouterNew)
		log.Info("router reported link added")
	} else {
		network.NotifyLinkEvent(link, event.LinkFromRouterKnown)
		log.Info("router reported link already known")
	}
}

func (network *Network) LinkConnected(msg *ctrl_pb.LinkConnected) error {
	if l, found := network.linkController.get(msg.Id); found {
		if state := l.CurrentState(); state.Mode != Pending {
			return errors.Errorf("link [l/%v] state is %v, not pending, cannot mark connected", msg.Id, state.Mode)
		}

		l.SetState(Connected)
		network.NotifyLinkConnected(l, msg)
		return nil
	}
	return errors.Errorf("no such link [l/%s]", msg.Id)
}

func (network *Network) LinkFaulted(l *Link, dupe bool) error {
	l.SetState(Failed)
	if dupe {
		network.NotifyLinkEvent(l, event.LinkDuplicate)
	} else {
		network.NotifyLinkEvent(l, event.LinkFault)
	}
	pfxlog.Logger().WithField("linkId", l.Id).Info("removing failed link")
	network.linkController.remove(l)
	return nil
}

func (network *Network) VerifyRouter(routerId string, fingerprints []string) error {
	router, err := network.GetRouter(routerId)
	if err != nil {
		return err
	}

	routerFingerprint := router.Fingerprint
	if routerFingerprint == nil {
		return errors.Errorf("invalid router %v, not yet enrolled", routerId)
	}

	for _, fp := range fingerprints {
		if fp == *routerFingerprint {
			return nil
		}
	}

	return errors.Errorf("could not verify fingerprint for router %v", routerId)
}

func (network *Network) RerouteLink(l *Link) {
	// This is called from Channel.rxer() and thus may not block
	go func() {
		network.handleRerouteLink(l)
	}()
}

func (network *Network) CreateCircuit(params CreateCircuitParams) (*Circuit, error) {
	clientId := params.GetClientId()
	service := params.GetServiceId()
	ctx := params.GetLogContext()
	deadline := params.GetDeadline()

	startTime := time.Now()

	instanceId, serviceId := parseInstanceIdAndService(service)

	// 1: Allocate Circuit Identifier
	circuitId, err := network.circuitController.nextCircuitId()
	if err != nil {
		network.CircuitFailedEvent(circuitId, params, startTime, nil, nil, CircuitFailureIdGenerationError)
		return nil, err
	}
	ctx.WithFields(map[string]interface{}{
		"circuitId":     circuitId,
		"serviceId":     service,
		"attemptNumber": 1,
	})
	logger := pfxlog.ChannelLogger(logcontext.SelectPath).Wire(ctx).Entry

	attempt := uint32(0)
	allCleanups := make(map[string]struct{})
	rs := network.newRouteSender(circuitId)
	defer func() { network.removeRouteSender(rs) }()
	for {
		// 2: Find Service
		svc, err := network.Services.Read(serviceId)
		if err != nil {
			network.CircuitFailedEvent(circuitId, params, startTime, nil, nil, CircuitFailureInvalidService)
			network.ServiceDialOtherError(serviceId)
			return nil, err
		}
		logger = logger.WithField("serviceName", svc.Name)

		// 3: select terminator
		strategy, terminator, pathNodes, strategyData, circuitErr := network.selectPath(params, svc, instanceId, ctx)
		if circuitErr != nil {
			network.CircuitFailedEvent(circuitId, params, startTime, nil, nil, circuitErr.Cause())
			network.ServiceDialOtherError(serviceId)
			return nil, circuitErr
		}

		// 4: Create Path
		path, pathErr := network.CreatePathWithNodes(pathNodes)
		if pathErr != nil {
			network.CircuitFailedEvent(circuitId, params, startTime, nil, terminator, pathErr.Cause())
			network.ServiceDialOtherError(serviceId)
			return nil, pathErr
		}

		// get circuit tags
		tags := params.GetCircuitTags(terminator)

		// 4a: Create Route Messages
		rms := path.CreateRouteMessages(attempt, circuitId, terminator, deadline)
		rms[len(rms)-1].Egress.PeerData = clientId.Data
		for _, msg := range rms {
			msg.Context = &ctrl_pb.Context{
				Fields:      ctx.GetStringFields(),
				ChannelMask: ctx.GetChannelsMask(),
			}
			msg.Tags = tags
		}

		// 5: Routing
		logger.Debug("route attempt for circuit")
		peerData, cleanups, circuitErr := rs.route(attempt, path, rms, strategy, terminator, ctx.Clone())
		for k, v := range cleanups {
			allCleanups[k] = v
		}
		if circuitErr != nil {
			logger.WithError(circuitErr).Warn("route attempt for circuit failed")
			network.CircuitFailedEvent(circuitId, params, startTime, path, terminator, circuitErr.Cause())
			attempt++
			ctx.WithField("attemptNumber", attempt+1)
			logger = logger.WithField("attemptNumber", attempt+1)
			if attempt < network.options.CreateCircuitRetries {
				continue
			} else {
				// revert successful routes
				logger.Warnf("circuit creation failed after [%d] attempts, sending cleanup unroutes", network.options.CreateCircuitRetries)
				for cleanupRId := range allCleanups {
					if r := network.GetConnectedRouter(cleanupRId); r != nil {
						if err := sendUnroute(r, circuitId, true); err == nil {
							logger.WithField("routerId", cleanupRId).Debug("sent cleanup unroute for circuit")
						} else {
							logger.WithField("routerId", cleanupRId).Error("error sending cleanup unroute for circuit")
						}
					} else {
						logger.WithField("routerId", cleanupRId).Error("router for circuit cleanup not connected")
					}
				}

				return nil, errors.Wrapf(circuitErr, "exceeded maximum [%d] retries creating circuit [c/%s]", network.options.CreateCircuitRetries, circuitId)
			}
		}

		// 5.a: Unroute Abandoned Routers (from Previous Attempts)
		usedRouters := make(map[string]struct{})
		for _, r := range path.Nodes {
			usedRouters[r.Id] = struct{}{}
		}
		cleanupCount := 0
		for cleanupRId := range allCleanups {
			if _, found := usedRouters[cleanupRId]; !found {
				cleanupCount++
				if r := network.GetConnectedRouter(cleanupRId); r != nil {
					if err := sendUnroute(r, circuitId, true); err == nil {
						logger.WithField("routerId", cleanupRId).Debug("sent abandoned cleanup unroute for circuit to router")
					} else {
						logger.WithField("routerId", cleanupRId).WithError(err).Error("error sending abandoned cleanup unroute for circuit to router")
					}
				} else {
					logger.WithField("routerId", cleanupRId).Error("router not connected for circuit, abandoned cleanup")
				}
			}
		}
		logger.Debugf("cleaned up [%d] abandoned routers for circuit", cleanupCount)

		path.InitiatorLocalAddr = string(clientId.Data[uint32(ctrl_msg.InitiatorLocalAddressHeader)])
		path.InitiatorRemoteAddr = string(clientId.Data[uint32(ctrl_msg.InitiatorRemoteAddressHeader)])
		path.TerminatorLocalAddr = string(peerData[uint32(ctrl_msg.TerminatorLocalAddressHeader)])
		path.TerminatorRemoteAddr = string(peerData[uint32(ctrl_msg.TerminatorRemoteAddressHeader)])

		delete(peerData, uint32(ctrl_msg.InitiatorLocalAddressHeader))
		delete(peerData, uint32(ctrl_msg.InitiatorRemoteAddressHeader))
		delete(peerData, uint32(ctrl_msg.TerminatorLocalAddressHeader))
		delete(peerData, uint32(ctrl_msg.TerminatorRemoteAddressHeader))

		for k, v := range strategyData {
			peerData[k] = v
		}

		now := time.Now()
		// 6: Create Circuit Object
		circuit := &Circuit{
			Id:         circuitId,
			ClientId:   clientId.Token,
			ServiceId:  svc.Id,
			Path:       path,
			Terminator: terminator,
			PeerData:   peerData,
			CreatedAt:  now,
			UpdatedAt:  now,
			Tags:       tags,
		}
		network.circuitController.add(circuit)
		creationTimespan := time.Since(startTime)
		network.CircuitEvent(event.CircuitCreated, circuit, &creationTimespan)

		logger.WithField("path", circuit.Path).
			WithField("terminator_local_address", circuit.Path.TerminatorLocalAddr).
			WithField("terminator_remote_address", circuit.Path.TerminatorRemoteAddr).
			Debug("created circuit")
		return circuit, nil
	}
}

func (network *Network) ReportForwardingFaults(ffr *ForwardingFaultReport) {
	select {
	case network.forwardingFaults <- struct{}{}:
	default:
	}

	go network.handleForwardingFaults(ffr)
}

func parseInstanceIdAndService(service string) (string, string) {
	atIndex := strings.IndexRune(service, '@')
	if atIndex < 0 {
		return "", service
	}
	identityId := service[0:atIndex]
	serviceId := service[atIndex+1:]
	return identityId, serviceId
}

func (network *Network) selectPath(params CreateCircuitParams, svc *Service, instanceId string, ctx logcontext.Context) (xt.Strategy, xt.CostedTerminator, []*Router, xt.PeerData, CircuitError) {
	paths := map[string]*PathAndCost{}
	var weightedTerminators []xt.CostedTerminator
	var errList []error

	log := pfxlog.ChannelLogger(logcontext.SelectPath).Wire(ctx)

	hasOfflineRouters := false
	pathError := false

	for _, terminator := range svc.Terminators {
		if terminator.InstanceId != instanceId {
			continue
		}

		pathAndCost, found := paths[terminator.Router]
		if !found {
			dstR := network.Routers.getConnected(terminator.GetRouterId())
			if dstR == nil {
				err := errors.Errorf("router with id=%v on terminator with id=%v for service name=%v is not online",
					terminator.GetRouterId(), terminator.GetId(), svc.Name)
				log.Debugf("error while calculating path for service %v: %v", svc.Id, err)

				errList = append(errList, err)
				hasOfflineRouters = true
				continue
			}

			path, cost, err := network.shortestPath(params.GetSourceRouter(), dstR)
			if err != nil {
				log.Debugf("error while calculating path for service %v: %v", svc.Id, err)
				errList = append(errList, err)
				pathError = true
				continue
			}

			pathAndCost = newPathAndCost(path, cost)
			paths[terminator.GetRouterId()] = pathAndCost
		}

		dynamicCost := xt.GlobalCosts().GetDynamicCost(terminator.Id)
		unbiasedCost := uint32(terminator.Cost) + uint32(dynamicCost) + pathAndCost.cost
		biasedCost := terminator.Precedence.GetBiasedCost(unbiasedCost)
		costedTerminator := &RoutingTerminator{
			Terminator: terminator,
			RouteCost:  biasedCost,
		}
		weightedTerminators = append(weightedTerminators, costedTerminator)
	}

	if len(svc.Terminators) == 0 {
		return nil, nil, nil, nil, newCircuitErrorf(CircuitFailureNoTerminators, "service %v has no terminators", svc.Id)
	}

	if len(weightedTerminators) == 0 {
		if pathError {
			return nil, nil, nil, nil, newCircuitErrWrap(CircuitFailureNoPath, errorz.MultipleErrors(errList))
		}

		if hasOfflineRouters {
			return nil, nil, nil, nil, newCircuitErrorf(CircuitFailureNoOnlineTerminators, "service %v has no online terminators for instanceId %v", svc.Id, instanceId)
		}

		return nil, nil, nil, nil, newCircuitErrorf(CircuitFailureNoTerminators, "service %v has no terminators for instanceId %v", svc.Id, instanceId)
	}

	strategy, err := network.strategyRegistry.GetStrategy(svc.TerminatorStrategy)
	if err != nil {
		return nil, nil, nil, nil, newCircuitErrWrap(CircuitFailureInvalidStrategy, err)
	}

	sort.Slice(weightedTerminators, func(i, j int) bool {
		return weightedTerminators[i].GetRouteCost() < weightedTerminators[j].GetRouteCost()
	})

	terminator, peerData, err := strategy.Select(params, weightedTerminators)

	if err != nil {
		return nil, nil, nil, nil, newCircuitErrorf(CircuitFailureStrategyError, "strategy %v errored selecting terminator for service %v: %v", svc.TerminatorStrategy, svc.Id, err)
	}

	if terminator == nil {
		return nil, nil, nil, nil, newCircuitErrorf(CircuitFailureStrategyError, "strategy %v did not select terminator for service %v", svc.TerminatorStrategy, svc.Id)
	}

	path := paths[terminator.GetRouterId()].path

	if log.Logger.IsLevelEnabled(logrus.DebugLevel) {
		buf := strings.Builder{}
		buf.WriteString("[")
		if len(weightedTerminators) > 0 {
			buf.WriteString(fmt.Sprintf("%v: %v", weightedTerminators[0].GetId(), weightedTerminators[0].GetRouteCost()))
			for _, t := range weightedTerminators[1:] {
				buf.WriteString(", ")
				buf.WriteString(fmt.Sprintf("%v: %v", t.GetId(), t.GetRouteCost()))
			}
		}
		buf.WriteString("]")
		var routerIds []string
		for _, r := range path {
			routerIds = append(routerIds, fmt.Sprintf("r/%s", r.Id))
		}
		pathStr := strings.Join(routerIds, "->")
		log.Debugf("selected terminator %v for path %v from %v", terminator.GetId(), pathStr, buf.String())
	}

	return strategy, terminator, path, peerData, nil
}

func (network *Network) RemoveCircuit(circuitId string, now bool) error {
	log := pfxlog.Logger().WithField("circuitId", circuitId)

	if circuit, found := network.circuitController.get(circuitId); found {
		for _, r := range circuit.Path.Nodes {
			err := sendUnroute(r, circuit.Id, now)
			if err != nil {
				log.Errorf("error sending unroute to [r/%s] (%s)", r.Id, err)
			}
		}

		network.circuitController.remove(circuit)
		network.CircuitEvent(event.CircuitDeleted, circuit, nil)

		if svc, err := network.Services.Read(circuit.ServiceId); err == nil {
			if strategy, err := network.strategyRegistry.GetStrategy(svc.TerminatorStrategy); strategy != nil {
				strategy.NotifyEvent(xt.NewCircuitRemoved(circuit.Terminator))
			} else if err != nil {
				log.WithError(err).WithField("terminatorStrategy", svc.TerminatorStrategy).Warn("failed to notify strategy of circuit end, invalid strategy")
			}
		} else {
			log.WithError(err).Error("unable to get service for circuit")
		}

		log.Debug("removed circuit")

		return nil
	}
	return InvalidCircuitError{circuitId: circuitId}
}

func (network *Network) CreatePath(srcR, dstR *Router) (*Path, error) {
	ingressId, err := network.sequence.NextHash()
	if err != nil {
		return nil, err
	}

	egressId, err := network.sequence.NextHash()
	if err != nil {
		return nil, err
	}

	path := &Path{
		Links:     make([]*Link, 0),
		IngressId: ingressId,
		EgressId:  egressId,
		Nodes:     make([]*Router, 0),
	}
	path.Nodes = append(path.Nodes, srcR)
	path.Nodes = append(path.Nodes, dstR)

	return network.UpdatePath(path)
}

func (network *Network) CreatePathWithNodes(nodes []*Router) (*Path, CircuitError) {
	ingressId, err := network.sequence.NextHash()
	if err != nil {
		return nil, newCircuitErrWrap(CircuitFailureIdGenerationError, err)
	}

	egressId, err := network.sequence.NextHash()
	if err != nil {
		return nil, newCircuitErrWrap(CircuitFailureIdGenerationError, err)
	}

	path := &Path{
		Nodes:     nodes,
		IngressId: ingressId,
		EgressId:  egressId,
	}
	if err := network.setLinks(path); err != nil {
		return nil, newCircuitErrWrap(CircuitFailurePathMissingLink, err)
	}
	return path, nil
}

func (network *Network) UpdatePath(path *Path) (*Path, error) {
	srcR := path.Nodes[0]
	dstR := path.Nodes[len(path.Nodes)-1]
	nodes, _, err := network.shortestPath(srcR, dstR)
	if err != nil {
		return nil, err
	}

	path2 := &Path{
		Nodes:                nodes,
		IngressId:            path.IngressId,
		EgressId:             path.EgressId,
		InitiatorLocalAddr:   path.InitiatorLocalAddr,
		InitiatorRemoteAddr:  path.InitiatorRemoteAddr,
		TerminatorLocalAddr:  path.TerminatorLocalAddr,
		TerminatorRemoteAddr: path.TerminatorRemoteAddr,
	}
	if err := network.setLinks(path2); err != nil {
		return nil, err
	}
	return path2, nil
}

func (network *Network) setLinks(path *Path) error {
	if len(path.Nodes) > 1 {
		for i := 0; i < len(path.Nodes)-1; i++ {
			if link, found := network.linkController.leastExpensiveLink(path.Nodes[i], path.Nodes[i+1]); found {
				path.Links = append(path.Links, link)
			} else {
				return errors.Errorf("no link from r/%v to r/%v", path.Nodes[i].Id, path.Nodes[i+1].Id)
			}
		}
	}
	return nil
}

func (network *Network) AddRouterPresenceHandler(h RouterPresenceHandler) {
	network.routerPresenceHandlers = append(network.routerPresenceHandlers, h)
}

func (network *Network) Run() {
	defer logrus.Info("exited")
	logrus.Info("started")

	go network.watchdog()

	ticker := time.NewTicker(time.Duration(network.options.CycleSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-network.assembleAndCleanC:
			network.assemble()
			network.clean()

		case <-network.forwardingFaults:
			network.clean()

		case <-ticker.C:
			network.assemble()
			network.clean()
			network.smart()
			network.linkController.scanForDeadLinks()

		case <-network.closeNotify:
			network.eventDispatcher.RemoveMetricsMessageHandler(network)
			network.metricsRegistry.DisposeAll()
			return
		}

		// notify the watchdog that we're processing
		select {
		case network.watchdogCh <- struct{}{}:
		default:
		}
	}
}

func (network *Network) watchdog() {
	watchdogInterval := 2 * time.Duration(network.options.CycleSeconds) * time.Second
	consecutiveFails := 0
	for {
		// check every 2x cycle seconds
		time.Sleep(watchdogInterval)
		select {
		case <-network.watchdogCh:
			consecutiveFails = 0
			continue
		case <-network.closeNotify:
			return
		default:
			consecutiveFails++
			// network.Run didn't complete, something is stalling it
			pfxlog.Logger().
				WithField("watchdogInterval", watchdogInterval.String()).
				WithField("consecutiveFails", consecutiveFails).
				Warn("network.Run did not finish within watchdog interval")

			if consecutiveFails == 3 {
				debugz.DumpStack()
			}
		}
	}
}

func (network *Network) handleRerouteLink(l *Link) {
	log := logrus.WithField("linkId", l.Id)
	log.Info("changed link")
	if err := network.rerouteLink(l, time.Now().Add(DefaultOptionsRouteTimeout)); err != nil {
		log.WithError(err).Error("unexpected error rerouting link")
	}
}

func (network *Network) handleForwardingFaults(ffr *ForwardingFaultReport) {
	network.fault(ffr)
}

func (network *Network) AddCapability(capability string) {
	network.lock.Lock()
	defer network.lock.Unlock()
	network.capabilities = append(network.capabilities, capability)
}

func (network *Network) GetCapabilities() []string {
	network.lock.Lock()
	defer network.lock.Unlock()
	return network.capabilities
}

func (network *Network) RemoveLink(linkId string) {
	log := pfxlog.Logger().WithField("linkId", linkId)

	link, _ := network.linkController.get(linkId)
	var iteration uint32

	var routerList []*Router
	if link != nil {
		iteration = link.Iteration
		routerList = []*Router{link.Src}
		if dst := link.GetDest(); dst != nil {
			routerList = append(routerList, dst)
		}
		log = log.WithField("srcRouterId", link.Src.Id).
			WithField("dstRouterId", link.DstId).
			WithField("iteration", iteration)
		log.Info("deleting known link")
	} else {
		routerList = network.AllConnectedRouters()
		log.Info("deleting unknown link (sending link fault to all connected routers)")
	}

	for _, router := range routerList {
		fault := &ctrl_pb.Fault{
			Subject:   ctrl_pb.FaultSubject_LinkFault,
			Id:        linkId,
			Iteration: iteration,
		}

		if ctrl := router.Control; ctrl != nil {
			if err := protobufs.MarshalTyped(fault).WithTimeout(15 * time.Second).Send(ctrl); err != nil {
				log.WithField("faultDestRouterId", router.Id).WithError(err).
					Error("failed to send link fault to router on link removal")
			} else {
				log.WithField("faultDestRouterId", router.Id).WithError(err).
					Info("sent link fault to router on link removal")
			}
		}
	}

	if link != nil {
		network.linkController.remove(link)
		network.RerouteLink(link)
	}
}

func (network *Network) rerouteLink(l *Link, deadline time.Time) error {
	circuits := network.circuitController.all()
	for _, circuit := range circuits {
		if circuit.Path.usesLink(l) {
			log := logrus.WithField("linkId", l.Id).
				WithField("circuitId", circuit.Id)
			log.Info("circuit uses link")
			if err := network.rerouteCircuit(circuit, deadline); err != nil {
				log.WithError(err).Error("error rerouting circuit, removing")
				if err := network.RemoveCircuit(circuit.Id, true); err != nil {
					log.WithError(err).Error("error removing circuit after reroute failure")
				}
			}
		}
	}

	return nil
}

func (network *Network) rerouteCircuitWithTries(circuit *Circuit, retries int) bool {
	log := pfxlog.Logger().WithField("circuitId", circuit.Id)

	for i := 0; i < retries; i++ {
		deadline := time.Now().Add(DefaultOptionsRouteTimeout)
		err := network.rerouteCircuit(circuit, deadline)
		if err == nil {
			return true
		}

		log.WithError(err).WithField("attempt", i).Error("error re-routing circuit")
	}

	if err := network.RemoveCircuit(circuit.Id, true); err != nil {
		log.WithError(err).Error("failure while removing circuit after failed re-route attempt")
	}
	return false
}

func (network *Network) rerouteCircuit(circuit *Circuit, deadline time.Time) error {
	log := pfxlog.Logger().WithField("circuitId", circuit.Id)
	if circuit.Rerouting.CompareAndSwap(false, true) {
		defer circuit.Rerouting.Store(false)

		log.Warn("rerouting circuit")

		if cq, err := network.UpdatePath(circuit.Path); err == nil {
			circuit.Path = cq
			circuit.UpdatedAt = time.Now()

			rms := cq.CreateRouteMessages(SmartRerouteAttempt, circuit.Id, circuit.Terminator, deadline)

			for i := 0; i < len(cq.Nodes); i++ {
				if _, err := sendRoute(cq.Nodes[i], rms[i], network.options.RouteTimeout); err != nil {
					log.WithError(err).Errorf("error sending route to [r/%s]", cq.Nodes[i].Id)
				}
			}

			log.Info("rerouted circuit")

			network.CircuitEvent(event.CircuitUpdated, circuit, nil)
			return nil
		} else {
			return err
		}
	} else {
		log.Info("not rerouting circuit, already in progress")
		return nil
	}
}

func (network *Network) smartReroute(circuit *Circuit, cq *Path, deadline time.Time) bool {
	retry := false
	log := pfxlog.Logger().WithField("circuitId", circuit.Id)
	if circuit.Rerouting.CompareAndSwap(false, true) {
		defer circuit.Rerouting.Store(false)

		circuit.Path = cq
		circuit.UpdatedAt = time.Now()

		rms := cq.CreateRouteMessages(SmartRerouteAttempt, circuit.Id, circuit.Terminator, deadline)

		for i := 0; i < len(cq.Nodes); i++ {
			if _, err := sendRoute(cq.Nodes[i], rms[i], network.options.RouteTimeout); err != nil {
				retry = true
				log.WithField("routerId", cq.Nodes[i].Id).WithError(err).Error("error sending smart route update to router")
				break
			}
		}

		if !retry {
			logrus.Debug("rerouted circuit")
			network.CircuitEvent(event.CircuitUpdated, circuit, nil)
		}
	}
	return retry
}

func (network *Network) AcceptMetricsMsg(metrics *metrics_pb.MetricsMessage) {
	if metrics.SourceId == network.nodeId {
		return // ignore metrics coming from the controller itself
	}

	log := pfxlog.Logger()

	router, err := network.Routers.Read(metrics.SourceId)
	if err != nil {
		log.Debugf("could not find router [r/%s] while processing metrics", metrics.SourceId)
		return
	}

	for _, link := range network.GetAllLinksForRouter(router.Id) {
		metricId := "link." + link.Id + ".latency"
		var latencyCost int64
		var found bool
		if latency, ok := metrics.Histograms[metricId]; ok {
			latencyCost = int64(latency.Mean)
			found = true

			metricId = "link." + link.Id + ".queue_time"
			if queueTime, ok := metrics.Histograms[metricId]; ok {
				latencyCost += int64(queueTime.Mean)
			}
		}

		if found {
			if link.Src.Id == router.Id {
				link.SetSrcLatency(latencyCost) // latency is in nanoseconds
			} else if link.DstId == router.Id {
				link.SetDstLatency(latencyCost) // latency is in nanoseconds
			} else {
				log.Warnf("link not for router")
			}
		}
	}
}

func sendRoute(r *Router, createMsg *ctrl_pb.Route, timeout time.Duration) (xt.PeerData, error) {
	log := pfxlog.Logger().WithField("routerId", r.Id).
		WithField("circuitId", createMsg.CircuitId)

	log.Debug("sending create route message")

	msg, err := protobufs.MarshalTyped(createMsg).WithTimeout(timeout).SendForReply(r.Control)
	if err != nil {
		log.WithError(err).WithField("timeout", timeout).Error("error sending route message")
		return nil, err
	}

	if msg.ContentType == ctrl_msg.RouteResultType {
		_, success := msg.Headers[ctrl_msg.RouteResultSuccessHeader]
		if !success {
			message := "route error, but no error message from router"
			if errMsg, found := msg.Headers[ctrl_msg.RouteResultErrorHeader]; found {
				message = string(errMsg)
			}
			return nil, errors.New(message)
		}

		peerData := xt.PeerData{}
		for k, v := range msg.Headers {
			if k > 0 {
				peerData[uint32(k)] = v
			}
		}

		return peerData, nil
	}
	return nil, fmt.Errorf("unexpected response type %v received in reply to route request", msg.ContentType)
}

func sendUnroute(r *Router, circuitId string, now bool) error {
	unroute := &ctrl_pb.Unroute{
		CircuitId: circuitId,
		Now:       now,
	}
	return protobufs.MarshalTyped(unroute).Send(r.Control)
}

func (network *Network) showOptions() {
	if jsonOptions, err := json.MarshalIndent(network.options, "", "  "); err == nil {
		pfxlog.Logger().Infof("network = %s", string(jsonOptions))
	} else {
		panic(err)
	}
}

type renderConfig interface {
	RenderJsonConfig() (string, error)
}

func (network *Network) Inspect(name string) (*string, error) {
	lc := strings.ToLower(name)

	if lc == "stackdump" {
		result := debugz.GenerateStack()
		return &result, nil
	} else if strings.HasPrefix(lc, "metrics") {
		msg := network.metricsRegistry.Poll()
		js, err := json.Marshal(msg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal metrics to json")
		}
		result := string(js)
		return &result, nil
	} else if lc == "config" {
		if rc, ok := network.config.(renderConfig); ok {
			val, err := rc.RenderJsonConfig()
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal config to json")
			}
			return &val, nil
		}
	} else if lc == "cluster-config" {
		if src, ok := network.Dispatcher.(renderConfig); ok {
			val, err := src.RenderJsonConfig()
			return &val, err
		}
	} else if lc == "connected-routers" {
		var result []map[string]any
		for _, r := range network.Routers.allConnected() {
			status := map[string]any{}
			status["Id"] = r.Id
			status["Name"] = r.Name
			status["Version"] = r.VersionInfo.Version
			status["ConnectTime"] = r.ConnectTime.Format(time.RFC3339)
			result = append(result, status)
		}
		val, err := json.Marshal(result)
		strVal := string(val)
		return &strVal, err
	} else if lc == "connected-peers" {
		if raftController, ok := network.Dispatcher.(*raft.Controller); ok {
			members, err := raftController.ListMembers()
			if err != nil {
				return nil, err
			}
			result, err := json.Marshal(members)
			if err != nil {
				return nil, fmt.Errorf("failed to marshall cluster member list to json (%w)", err)
			}
			resultStr := string(result)
			return &resultStr, nil
		}
	} else if lc == "router-messaging" {
		routerMessagingState, err := network.Managers.RouterMessaging.Inspect()
		if err != nil {
			return nil, err
		}
		result, err := json.Marshal(routerMessagingState)
		if err != nil {
			return nil, fmt.Errorf("failed to marshall router messaging state to json (%w)", err)
		}
		resultStr := string(result)
		return &resultStr, nil
	} else {
		for _, inspectTarget := range network.inspectionTargets.Value() {
			if handled, val, err := inspectTarget(lc); handled {
				return val, err
			}
		}
	}

	return nil, nil
}

func (network *Network) routerDeleted(routerId string) {
	circuits := network.GetAllCircuits()
	for _, circuit := range circuits {
		if circuit.HasRouter(routerId) {
			path := circuit.Path

			// If we're either the initiator, terminator (or both), cleanup the circuit since
			// we won't be able to re-establish it, and we'll never get a circuit fault
			if path.Nodes[0].Id == routerId || path.Nodes[len(path.Nodes)-1].Id == routerId {
				if err := network.RemoveCircuit(circuit.Id, true); err != nil {
					pfxlog.Logger().WithField("routerId", routerId).
						WithField("circuitId", circuit.Id).
						WithError(err).Error("unable to remove circuit after router was deleted")
				}
			}
		}
	}
}

var DbSnapshotTooFrequentError = dbSnapshotTooFrequentError{}

type dbSnapshotTooFrequentError struct{}

func (d dbSnapshotTooFrequentError) Error() string {
	return "may snapshot database at most once per minute"
}

func (network *Network) SnapshotDatabase() error {
	network.lock.Lock()
	defer network.lock.Unlock()

	if network.lastSnapshot.Add(time.Minute).After(time.Now()) {
		return DbSnapshotTooFrequentError
	}
	pfxlog.Logger().Info("snapshotting database")
	err := network.GetDb().View(func(tx *bbolt.Tx) error {
		return network.GetDb().Snapshot(tx)
	})
	if err == nil {
		network.lastSnapshot = time.Now()
	}
	return err
}

func (network *Network) SnapshotDatabaseToFile(path string) (string, error) {
	network.lock.Lock()
	defer network.lock.Unlock()

	if network.lastSnapshot.Add(time.Minute).After(time.Now()) {
		return "", DbSnapshotTooFrequentError
	}

	actualPath := path
	if actualPath == "" {
		actualPath = "DB_DIR/DB_FILE-DATE-TIME"
	}

	err := network.GetDb().View(func(tx *bbolt.Tx) error {
		now := time.Now()
		dateStr := now.Format("20060102")
		timeStr := now.Format("150405")
		actualPath = strings.ReplaceAll(actualPath, "DATE", dateStr)
		actualPath = strings.ReplaceAll(actualPath, "TIME", timeStr)

		currentIndex := db.LoadCurrentRaftIndex(tx)
		actualPath = strings.ReplaceAll(actualPath, "RAFT_INDEX", fmt.Sprintf("%v", currentIndex))
		actualPath = strings.ReplaceAll(actualPath, "DB_DIR", filepath.Dir(tx.DB().Path()))
		actualPath = strings.ReplaceAll(actualPath, "DB_FILE", filepath.Base(tx.DB().Path()))

		pfxlog.Logger().WithField("path", actualPath).Info("snapshotting database to file")

		_, err := os.Stat(actualPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			pfxlog.Logger().Infof("bolt db backup already exists: %v", actualPath)
			return errors.Errorf("bolt db backup already exists [%s]", actualPath)
		}

		file, err := os.OpenFile(actualPath, os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		defer func() {
			if err = file.Close(); err != nil {
				pfxlog.Logger().WithError(err).Errorf("failed to close backup database file %v", actualPath)
			}
		}()

		_, err = tx.WriteTo(file)
		return err
	})

	if err == nil {
		network.lastSnapshot = time.Now()
	}
	return actualPath, err
}

func (network *Network) RestoreSnapshot(cmd *command.SyncSnapshotCommand) error {
	log := pfxlog.Logger()
	currentSnapshotId, err := network.getDb().GetSnapshotId()
	if err != nil {
		log.WithError(err).Error("unable to get current snapshot id")
	}
	if currentSnapshotId != nil && *currentSnapshotId == cmd.SnapshotId {
		log.WithField("snapshotId", cmd.SnapshotId).Info("snapshot already current, skipping reload")
		return nil
	}

	buf := bytes.NewBuffer(cmd.Snapshot)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return errors.Wrapf(err, "unable to create gz reader for reading migration snapshot during restore")
	}

	network.getDb().RestoreFromReader(reader)
	return nil
}

func (network *Network) SnapshotToRaft() error {
	buf := &bytes.Buffer{}
	gzWriter := gzip.NewWriter(buf)
	snapshotId, err := network.db.SnapshotToWriter(gzWriter)
	if err != nil {
		return err
	}

	if err = gzWriter.Close(); err != nil {
		return errors.Wrap(err, "error finishing gz compression of migration snapshot")
	}

	cmd := &command.SyncSnapshotCommand{
		SnapshotId:   snapshotId,
		Snapshot:     buf.Bytes(),
		SnapshotSink: network.RestoreSnapshot,
	}

	return network.Dispatch(cmd)
}

func (network *Network) AddInspectTarget(target InspectTarget) {
	network.inspectionTargets.Append(target)
}

type Cache interface {
	RemoveFromCache(id string)
}

func newPathAndCost(path []*Router, cost int64) *PathAndCost {
	if cost > (1 << 20) {
		cost = 1 << 20
	}
	return &PathAndCost{
		path: path,
		cost: uint32(cost),
	}
}

type PathAndCost struct {
	path []*Router
	cost uint32
}

type InvalidCircuitError struct {
	circuitId string
}

func (err InvalidCircuitError) Error() string {
	return fmt.Sprintf("invalid circuit (%s)", err.circuitId)
}
