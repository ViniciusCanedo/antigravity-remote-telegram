package cdp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/platform"
)

// BackgroundDetector represents the contract for background detectors managed in the pool.
type BackgroundDetector interface {
	IsActive() bool
	Start()
	Stop()
}

// ClientFactory allows injecting mock Client instances in unit tests.
type ClientFactory func(opts ClientOptions) PoolClient

// PoolClient defines the interface required by ConnectionPool for each workspace CDP client.
type PoolClient interface {
	IsConnected() bool
	DiscoverAndConnectForWorkspace(workspacePath string) (bool, error)
	Disconnect() error
	On(event string, handler EventHandler)
	Call(method string, params any) (json.RawMessage, error)
	GetContexts() []Context
	GetPrimaryContextID() *int
	WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}

// ConnectionPool manages independent Client instances per workspace.
type ConnectionPool struct {
	mu                   sync.Mutex
	connections          map[string]PoolClient
	approvalDetectors    map[string]BackgroundDetector
	errorPopupDetectors  map[string]BackgroundDetector
	planningDetectors    map[string]BackgroundDetector
	userMessageDetectors map[string]BackgroundDetector
	connectingCalls      map[string]*inflightConnect
	cdpOptions           ClientOptions
	clientFactory        ClientFactory
	listeners            map[string][]func(projectName string)
}

type inflightConnect struct {
	done chan struct{}
	cdp  PoolClient
	err  error
}

// NewConnectionPool creates a new ConnectionPool instance.
func NewConnectionPool(opts ClientOptions, factory ...ClientFactory) *ConnectionPool {
	f := func(o ClientOptions) PoolClient {
		return NewClient(o)
	}
	if len(factory) > 0 && factory[0] != nil {
		f = factory[0]
	}

	return &ConnectionPool{
		connections:          make(map[string]PoolClient),
		approvalDetectors:    make(map[string]BackgroundDetector),
		errorPopupDetectors:  make(map[string]BackgroundDetector),
		planningDetectors:    make(map[string]BackgroundDetector),
		userMessageDetectors: make(map[string]BackgroundDetector),
		connectingCalls:      make(map[string]*inflightConnect),
		cdpOptions:           opts,
		clientFactory:        f,
		listeners:            make(map[string][]func(projectName string)),
	}
}

// ExtractProjectName extracts the project/folder name from a workspace path.
func (p *ConnectionPool) ExtractProjectName(workspacePath string) string {
	name := platform.ExtractProjectNameFromPath(workspacePath)
	if name != "" {
		return name
	}
	return workspacePath
}

// GetOrConnect retrieves a cached connection for workspacePath or creates a new one.
func (p *ConnectionPool) GetOrConnect(workspacePath string) (PoolClient, error) {
	projectName := p.ExtractProjectName(workspacePath)

	p.mu.Lock()
	existing := p.connections[projectName]
	if existing != nil {
		if existing.IsConnected() {
			p.mu.Unlock()
			logger.Debug(fmt.Sprintf("[CdpConnectionPool] Re-validating existing connection for %q...", projectName))
			_, err := existing.DiscoverAndConnectForWorkspace(workspacePath)
			if err == nil {
				logger.Debug(fmt.Sprintf("[CdpConnectionPool] Existing connection for %q is still valid.", projectName))
				return existing, nil
			}
			logger.Warn(fmt.Sprintf("[CdpConnectionPool] Connection for %q dropped during re-validation: %v", projectName, err))
			_ = existing.Disconnect()
			p.mu.Lock()
			delete(p.connections, projectName)
			p.mu.Unlock()
		} else {
			logger.Debug(fmt.Sprintf("[CdpConnectionPool] Removing stale disconnected entry for %q", projectName))
			delete(p.connections, projectName)
			p.mu.Unlock()
		}
	} else {
		p.mu.Unlock()
	}

	p.mu.Lock()
	if existing := p.connections[projectName]; existing != nil && existing.IsConnected() {
		p.mu.Unlock()
		return existing, nil
	}
	if pending, ok := p.connectingCalls[projectName]; ok {
		p.mu.Unlock()
		<-pending.done
		return pending.cdp, pending.err
	}

	inflight := &inflightConnect{
		done: make(chan struct{}),
	}
	p.connectingCalls[projectName] = inflight
	p.mu.Unlock()

	cdp, err := p.createAndConnect(workspacePath, projectName)

	p.mu.Lock()
	inflight.cdp = cdp
	inflight.err = err
	close(inflight.done)
	delete(p.connectingCalls, projectName)
	p.mu.Unlock()

	return cdp, err
}

func (p *ConnectionPool) createAndConnect(workspacePath, projectName string) (PoolClient, error) {
	p.mu.Lock()
	old := p.connections[projectName]
	if old != nil {
		delete(p.connections, projectName)
	}
	p.mu.Unlock()
	if old != nil {
		_ = old.Disconnect()
	}

	cdp := p.clientFactory(p.cdpOptions)

	cdp.On("disconnected", func(any) {
		logger.Error(fmt.Sprintf("[CdpConnectionPool] Workspace %q disconnected", projectName))
		p.emit("workspace:disconnected", projectName)
	})

	cdp.On("reconnected", func(any) {
		logger.Info(fmt.Sprintf("[CdpConnectionPool] Workspace %q reconnected", projectName))
		p.emit("workspace:reconnected", projectName)
	})

	cdp.On("reconnectFailed", func(any) {
		logger.Error(fmt.Sprintf("[CdpConnectionPool] Reconnection failed for workspace %q. Removing from pool", projectName))
		p.emit("workspace:reconnectFailed", projectName)
		p.DisconnectWorkspace(projectName)
	})

	_, err := cdp.DiscoverAndConnectForWorkspace(workspacePath)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.connections[projectName] = cdp
	p.mu.Unlock()
	return cdp, nil
}

// GetConnected returns the connected PoolClient for projectName, or nil.
func (p *ConnectionPool) GetConnected(projectName string) PoolClient {
	p.mu.Lock()
	defer p.mu.Unlock()
	cdp := p.connections[projectName]
	if cdp != nil && cdp.IsConnected() {
		return cdp
	}
	return nil
}

// DisconnectWorkspace disconnects the given workspace and stops its detectors.
func (p *ConnectionPool) DisconnectWorkspace(projectName string) {
	p.mu.Lock()
	cdp := p.connections[projectName]
	delete(p.connections, projectName)

	appDet := p.approvalDetectors[projectName]
	delete(p.approvalDetectors, projectName)

	errDet := p.errorPopupDetectors[projectName]
	delete(p.errorPopupDetectors, projectName)

	planDet := p.planningDetectors[projectName]
	delete(p.planningDetectors, projectName)

	userDet := p.userMessageDetectors[projectName]
	delete(p.userMessageDetectors, projectName)
	p.mu.Unlock()

	if cdp != nil {
		if err := cdp.Disconnect(); err != nil {
			logger.Error(fmt.Sprintf("[CdpConnectionPool] Error while disconnecting %s: %v", projectName, err))
		}
	}
	if appDet != nil {
		appDet.Stop()
	}
	if errDet != nil {
		errDet.Stop()
	}
	if planDet != nil {
		planDet.Stop()
	}
	if userDet != nil {
		userDet.Stop()
	}
}

// DisconnectAll disconnects all active workspace connections and stops all detectors.
func (p *ConnectionPool) DisconnectAll() {
	p.mu.Lock()
	var names []string
	for name := range p.connections {
		names = append(names, name)
	}
	p.mu.Unlock()

	for _, name := range names {
		p.DisconnectWorkspace(name)
	}
}

// RegisterApprovalDetector registers an approval detector for projectName, stopping any previous one.
func (p *ConnectionPool) RegisterApprovalDetector(projectName string, detector BackgroundDetector) {
	p.mu.Lock()
	existing := p.approvalDetectors[projectName]
	p.approvalDetectors[projectName] = detector
	p.mu.Unlock()

	if existing != nil && existing.IsActive() {
		existing.Stop()
	}
}

// GetApprovalDetector retrieves the registered approval detector for projectName.
func (p *ConnectionPool) GetApprovalDetector(projectName string) BackgroundDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.approvalDetectors[projectName]
}

// RegisterErrorPopupDetector registers an error popup detector for projectName, stopping any previous one.
func (p *ConnectionPool) RegisterErrorPopupDetector(projectName string, detector BackgroundDetector) {
	p.mu.Lock()
	existing := p.errorPopupDetectors[projectName]
	p.errorPopupDetectors[projectName] = detector
	p.mu.Unlock()

	if existing != nil && existing.IsActive() {
		existing.Stop()
	}
}

// GetErrorPopupDetector retrieves the registered error popup detector for projectName.
func (p *ConnectionPool) GetErrorPopupDetector(projectName string) BackgroundDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.errorPopupDetectors[projectName]
}

// RegisterPlanningDetector registers a planning detector for projectName, stopping any previous one.
func (p *ConnectionPool) RegisterPlanningDetector(projectName string, detector BackgroundDetector) {
	p.mu.Lock()
	existing := p.planningDetectors[projectName]
	p.planningDetectors[projectName] = detector
	p.mu.Unlock()

	if existing != nil && existing.IsActive() {
		existing.Stop()
	}
}

// GetPlanningDetector retrieves the registered planning detector for projectName.
func (p *ConnectionPool) GetPlanningDetector(projectName string) BackgroundDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.planningDetectors[projectName]
}

// RegisterUserMessageDetector registers a user message detector for projectName, stopping any previous one.
func (p *ConnectionPool) RegisterUserMessageDetector(projectName string, detector BackgroundDetector) {
	p.mu.Lock()
	existing := p.userMessageDetectors[projectName]
	p.userMessageDetectors[projectName] = detector
	p.mu.Unlock()

	if existing != nil && existing.IsActive() {
		existing.Stop()
	}
}

// GetUserMessageDetector retrieves the registered user message detector for projectName.
func (p *ConnectionPool) GetUserMessageDetector(projectName string) BackgroundDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userMessageDetectors[projectName]
}

// GetActiveWorkspaceNames returns the list of workspace names that currently have active connections.
func (p *ConnectionPool) GetActiveWorkspaceNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var active []string
	for name, cdp := range p.connections {
		if cdp != nil && cdp.IsConnected() {
			active = append(active, name)
		}
	}
	return active
}

// On registers a listener for pool lifecycle events (e.g. 'workspace:disconnected').
func (p *ConnectionPool) On(event string, handler func(projectName string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listeners[event] = append(p.listeners[event], handler)
}

// Emit triggers handlers for a lifecycle event.
func (p *ConnectionPool) Emit(event string, projectName string) {
	p.emit(event, projectName)
}

func (p *ConnectionPool) emit(event string, projectName string) {
	p.mu.Lock()
	handlers := make([]func(string), len(p.listeners[event]))
	copy(handlers, p.listeners[event])
	p.mu.Unlock()

	for _, h := range handlers {
		h(projectName)
	}
}
