package nmplugin

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/graipher/network-manager-defguard-plugin/internal/defguard"
)

const (
	BusName                             = "org.freedesktop.NetworkManager.defguard"
	Interface                           = "org.freedesktop.NetworkManager.VPN.Plugin"
	PropertiesInterface                 = "org.freedesktop.DBus.Properties"
	ObjectPath          dbus.ObjectPath = "/org/freedesktop/NetworkManager/VPN/Plugin"
)

type ConnectionSettings = map[string]map[string]dbus.Variant
type VariantMap = map[string]dbus.Variant
type ServiceState uint32

const (
	StateUnknown ServiceState = iota
	StateInit
	StateShutdown
	StateStarting
	StateStarted
	StateStopping
	StateStopped
)

const (
	failureLogin uint32 = iota
	failureConnect
)

type Service struct {
	conn *dbus.Conn
	log  *log.Logger
	// ponytail: one active profile keeps the NetworkManager lifecycle unambiguous;
	// use per-activation state after multi-connection behavior is validated.
	mu     sync.Mutex
	state  ServiceState
	cancel context.CancelFunc
	active *activation
}

type activation struct {
	id            int64
	instance      string
	user          string
	trafficMode   string
	interfaceName string
	endpoint      string
	created       bool
	cancel        context.CancelFunc
	ctx           context.Context
}

func New(conn *dbus.Conn, logger *log.Logger) *Service {
	return &Service{conn: conn, log: logger, state: StateInit}
}

func (s *Service) Export() error {
	if err := s.conn.Export(s, ObjectPath, Interface); err != nil {
		return err
	}
	if err := s.conn.Export(&properties{s}, ObjectPath, PropertiesInterface); err != nil {
		return err
	}
	return s.conn.Export(introspect.Introspectable(IntrospectionXML), ObjectPath, "org.freedesktop.DBus.Introspectable")
}

func (s *Service) NeedSecrets(ConnectionSettings) (string, *dbus.Error) { return "", nil }

func (s *Service) Connect(settings ConnectionSettings) *dbus.Error {
	return s.connect(settings)
}

func (s *Service) ConnectInteractive(settings ConnectionSettings, _ VariantMap) *dbus.Error {
	return s.connect(settings)
}

func (s *Service) connect(settings ConnectionSettings) *dbus.Error {
	a, err := parseActivation(settings)
	if err != nil {
		return dbusFailure(err)
	}
	s.mu.Lock()
	if s.active != nil || s.cancel != nil {
		s.mu.Unlock()
		return dbus.NewError("org.freedesktop.NetworkManager.VPN.Error.AlreadyStarted", []any{"Defguard connection already active"})
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	a.ctx = ctx
	s.mu.Unlock()
	s.setState(StateStarting)
	go s.connectDefguard(ctx, a)
	return nil
}

func (s *Service) connectDefguard(ctx context.Context, a activation) {
	client := defguard.Client{Runner: defguard.CommandRunner{User: a.user}}
	list, err := client.List(ctx)
	if err != nil {
		s.fail(failureConnect, err)
		return
	}
	location, err := defguard.FindLocation(list, a.id, a.instance)
	if err != nil {
		s.fail(failureConnect, err)
		return
	}
	before, err := client.Status(ctx)
	if err != nil {
		s.fail(failureConnect, err)
		return
	}
	if err := client.Connect(ctx, a.id, a.instance, a.trafficMode); err != nil {
		s.fail(failureLogin, err)
		return
	}
	after, err := client.Status(ctx)
	if err != nil {
		s.fail(failureConnect, err)
		return
	}
	a.interfaceName, err = defguard.NewInterface(before, after, location.Name)
	if err != nil {
		s.fail(failureConnect, err)
		return
	}
	a.created = !defguard.HasInterface(before, a.interfaceName, location.Name)
	s.activate(ctx, a)
}

func (s *Service) activate(ctx context.Context, a activation) {
	if a.interfaceName == "" {
		s.failActivation(a, failureLogin, fmt.Errorf("Defguard authentication did not return an interface"))
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	status, err := (defguard.Client{Runner: defguard.CommandRunner{User: a.user}}).Status(callCtx)
	cancel()
	if err != nil || !defguard.HasInterface(status, a.interfaceName, "") {
		if err == nil {
			err = fmt.Errorf("interface %q is not active in Defguard", a.interfaceName)
		}
		s.failActivation(a, failureConnect, err)
		return
	}
	config := VariantMap{
		"tundev":  dbus.MakeVariant(a.interfaceName),
		"has-ip4": dbus.MakeVariant(false),
		"has-ip6": dbus.MakeVariant(false),
	}
	if gateway, err := gatewayVariant(ctx, a.endpoint); err == nil {
		config["gateway"] = gateway
	} else {
		s.failActivation(a, failureConnect, err)
		return
	}
	if ctx.Err() != nil {
		s.cleanupCreated(a)
		return
	}
	if err := s.conn.Emit(ObjectPath, Interface+".Config", config); err != nil {
		s.failActivation(a, failureConnect, err)
		return
	}
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	a.cancel = s.cancel
	s.active = &a
	s.cancel = nil
	s.mu.Unlock()
	s.setState(StateStarted)
	go s.monitor(ctx, a)
}

func (s *Service) failActivation(a activation, reason uint32, err error) {
	s.cleanupCreated(a)
	s.fail(reason, err)
}

func (s *Service) cleanupCreated(a activation) {
	if !a.created {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := (defguard.Client{Runner: defguard.CommandRunner{User: a.user}}).Disconnect(ctx, a.id, a.instance); err != nil {
		s.log.Printf("cleanup Defguard location %d: %v", a.id, err)
	}
}

func (s *Service) monitor(ctx context.Context, a activation) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := net.InterfaceByName(a.interfaceName); err != nil {
				s.externalStop(false)
				return
			}
		}
	}
}

func (s *Service) Disconnect() *dbus.Error {
	s.mu.Lock()
	a := s.active
	cancel := s.cancel
	s.active, s.cancel = nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if a == nil {
		s.setState(StateStopped)
		return nil
	}
	a.cancel()
	s.setState(StateStopping)
	ctx, stop := context.WithTimeout(context.Background(), 30*time.Second)
	defer stop()
	err := (defguard.Client{Runner: defguard.CommandRunner{User: a.user}}).Disconnect(ctx, a.id, a.instance)
	s.setState(StateStopped)
	if err != nil {
		return dbusFailure(err)
	}
	return nil
}

func (s *Service) NewSecrets(ConnectionSettings) *dbus.Error { return nil }
func (s *Service) SetConfig(VariantMap) *dbus.Error          { return nil }
func (s *Service) SetIp4Config(VariantMap) *dbus.Error       { return nil }
func (s *Service) SetIp6Config(VariantMap) *dbus.Error       { return nil }
func (s *Service) SetFailure(string) *dbus.Error             { return s.Disconnect() }

func (s *Service) setState(state ServiceState) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
	if err := s.conn.Emit(ObjectPath, Interface+".StateChanged", uint32(state)); err != nil {
		s.log.Printf("emit state: %v", err)
	}
}

func (s *Service) fail(reason uint32, err error) {
	s.log.Printf("activation failed: %v", err)
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel, s.active = nil, nil
	s.mu.Unlock()
	_ = s.conn.Emit(ObjectPath, Interface+".Failure", reason)
	s.setState(StateStopped)
}

func (s *Service) externalStop(failed bool) {
	s.mu.Lock()
	if s.active != nil {
		s.active.cancel()
	}
	s.active, s.cancel = nil, nil
	s.mu.Unlock()
	if failed {
		_ = s.conn.Emit(ObjectPath, Interface+".Failure", failureConnect)
	}
	s.setState(StateStopped)
}

func parseActivation(settings ConnectionSettings) (activation, error) {
	values := map[string]string{}
	var permittedUsers []string
	for _, section := range settings {
		for key, variant := range section {
			switch value := variant.Value().(type) {
			case string:
				values[key] = value
			case map[string]string:
				for k, v := range value {
					values[k] = v
				}
			case []string:
				if key == "permissions" {
					permittedUsers = permissionUsers(value)
				}
			}
		}
	}
	id, err := strconv.ParseInt(values["location-id"], 10, 64)
	if err != nil || id <= 0 {
		return activation{}, fmt.Errorf("invalid location-id")
	}
	username := values["user"]
	if username == "" {
		return activation{}, fmt.Errorf("missing profile user")
	}
	if _, err := user.Lookup(username); err != nil {
		return activation{}, fmt.Errorf("unknown profile user %q", username)
	}
	if len(permittedUsers) != 1 || permittedUsers[0] != username {
		return activation{}, fmt.Errorf("profile user %q does not match its NetworkManager permission", username)
	}
	trafficMode := values["traffic-mode"]
	if trafficMode == "" {
		trafficMode = "all"
	}
	if trafficMode != "all" && trafficMode != "predefined" {
		return activation{}, fmt.Errorf("invalid traffic mode %q", trafficMode)
	}
	return activation{id: id, instance: values["instance"], user: username, trafficMode: trafficMode, endpoint: values["endpoint"]}, nil
}

func permissionUsers(permissions []string) []string {
	var result []string
	for _, permission := range permissions {
		parts := strings.Split(permission, ":")
		if len(parts) >= 2 && parts[0] == "user" && parts[1] != "" {
			result = append(result, parts[1])
		}
	}
	return result
}

func gatewayVariant(ctx context.Context, endpoint string) (dbus.Variant, error) {
	host := strings.TrimSpace(endpoint)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	if host == "" {
		return dbus.Variant{}, fmt.Errorf("missing Defguard endpoint")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addrVariant(addr)
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return dbus.Variant{}, fmt.Errorf("resolve Defguard endpoint %q: %w", host, err)
	}
	return addrVariant(addresses[0])
}

func addrVariant(addr netip.Addr) (dbus.Variant, error) {
	addr = addr.Unmap()
	if addr.Is4() {
		b := addr.As4()
		return dbus.MakeVariant(binary.BigEndian.Uint32(b[:])), nil
	}
	if addr.Is6() {
		b := addr.As16()
		return dbus.MakeVariant(b[:]), nil
	}
	return dbus.Variant{}, fmt.Errorf("invalid endpoint address")
}

func dbusFailure(err error) *dbus.Error {
	return dbus.NewError("org.freedesktop.NetworkManager.VPN.Error.Failed", []any{err.Error()})
}

type properties struct{ service *Service }

func (p *properties) Get(iface, property string) (dbus.Variant, *dbus.Error) {
	if iface != Interface || property != "State" {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", nil)
	}
	p.service.mu.Lock()
	defer p.service.mu.Unlock()
	return dbus.MakeVariant(uint32(p.service.state)), nil
}
func (p *properties) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	v, err := p.Get(iface, "State")
	if err != nil {
		return nil, err
	}
	return map[string]dbus.Variant{"State": v}, nil
}
func (*properties) Set(string, string, dbus.Variant) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", nil)
}

const IntrospectionXML = `<node>
 <interface name="org.freedesktop.DBus.Introspectable"><method name="Introspect"><arg name="xml_data" type="s" direction="out"/></method></interface>
 <interface name="org.freedesktop.DBus.Properties"><method name="Get"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="out"/></method><method name="GetAll"><arg type="s" direction="in"/><arg type="a{sv}" direction="out"/></method><method name="Set"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="in"/></method></interface>
 <interface name="org.freedesktop.NetworkManager.VPN.Plugin">
  <method name="Connect"><arg name="connection" type="a{sa{sv}}" direction="in"/></method>
  <method name="ConnectInteractive"><arg name="connection" type="a{sa{sv}}" direction="in"/><arg name="details" type="a{sv}" direction="in"/></method>
  <method name="NeedSecrets"><arg name="settings" type="a{sa{sv}}" direction="in"/><arg name="setting_name" type="s" direction="out"/></method>
  <method name="NewSecrets"><arg name="connection" type="a{sa{sv}}" direction="in"/></method><method name="Disconnect"/>
  <method name="SetConfig"><arg type="a{sv}" direction="in"/></method><method name="SetIp4Config"><arg type="a{sv}" direction="in"/></method><method name="SetIp6Config"><arg type="a{sv}" direction="in"/></method><method name="SetFailure"><arg type="s" direction="in"/></method>
  <property name="State" type="u" access="read"/><signal name="StateChanged"><arg type="u"/></signal><signal name="Config"><arg type="a{sv}"/></signal><signal name="Failure"><arg type="u"/></signal>
 </interface>
</node>`
