package discovery

import (
	"net"

	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
)

type Membership struct {
	Config
	handler Handler
	serf    *serf.Serf
	events  chan serf.Event
	logger  *zap.Logger
}

type Config struct {
	// Unique identifier of the node in cluster,
	// determined by index in the cluster
	NodeName string
	// Serf listens on this address and port for gossiping :)
	BindAddr string
	// Simple data informing the cluster how to handle the nodes.
	// Example: Decide whether the node is a voter or non-voter,
	// changing the node's role in the Raft cluster.
	Tags map[string]string
	// Configure new nodes to join an existing cluster
	StartJoinAddrs []string
}

// Represent components in our service
// that needs to know when a server joins or leaves the cluster
type Handler interface {
	Join(name, addr string) error
	Leave(name string) error
}

func New(handler Handler, config Config) (*Membership, error) {
	m := &Membership{
		Config:  config,
		handler: handler,
		// TODO: Are we using the global logger here?
		logger: zap.L().Named("membership"),
	}
	if err := m.setupSerf(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Membership) setupSerf() (err error) {
	addr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
	if err != nil {
		return err
	}
	config := serf.DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = addr.IP.String()
	config.MemberlistConfig.BindPort = addr.Port
	m.events = make(chan serf.Event)
	// Receive Serf's events when a node joins or leaves the cluster
	config.EventCh = m.events
	config.Tags = m.Tags
	config.NodeName = m.Config.NodeName
	m.serf, err = serf.Create(config)
	if err != nil {
		return err
	}

	// Guess handling events in another goroutine
	// to ensure we don't mess it up with the main goroutine?
	go m.eventHandler()
	if m.StartJoinAddrs != nil {
		// After the new nodes connect to the nodes in the existing cluster,
		// it'll learn about the rest of the nodes.
		// Serf's gossip protocol takes care of the rest when joining the new nodes to the cluster.
		_, err = m.serf.Join(m.StartJoinAddrs, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Membership) eventHandler() {
	for e := range m.events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			for _, member := range e.(serf.MemberEvent).Members {
				// Here the local node receives its own join event
				// but it does not need to react to itself
				// so we keep unnecessary handling
				if m.isLocal(member) {
					continue
				}
				// At this point Serf has already informed all members
				// so here we tell nodes other than the local one to react
				m.handleJoin(member)
			}
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					// Nothing to do for this member?
					// and will this lead to other nodes not reacting to the change?
					return
				}
				m.handleLeave(member)
			}
		}
	}
}

func (m *Membership) Members() []serf.Member {
	return m.serf.Members()
}

func (m *Membership) Leave() error {
	return m.serf.Leave()
}

// Being local means the node itself
func (m *Membership) isLocal(member serf.Member) bool {
	return m.serf.LocalMember().Name == member.Name
}

func (m *Membership) handleJoin(member serf.Member) {
	// Send an event to ALL NODES!
	if err := m.handler.Join(
		member.Name,
		// Metadata to inform cluster how to handle member
		member.Tags["rpc_addr"],
	); err != nil {
		m.logError(err, "failed to join", member)
	}
}

func (m *Membership) handleLeave(member serf.Member) {
	if err := m.handler.Leave(
		member.Name,
	); err != nil {
		m.logError(err, "failed to leave", member)
	}
}

func (m *Membership) logError(err error, msg string, member serf.Member) {
	m.logger.Error(
		msg,
		zap.Error(err),
		zap.String("name", member.Name),
		zap.String("rpc_addr", member.Tags["rpc_addr"]),
	)
}
