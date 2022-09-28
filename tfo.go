// Package tfo provides TCP Fast Open support for the [net] dialer and listener.
//
// The dial functions have an additional buffer parameter, which specifies data in SYN.
// If the buffer is empty, TFO is not used.
//
// This package supports Linux, Windows, macOS, and FreeBSD.
// On unsupported platforms, [ErrPlatformUnsupported] is returned.
//
// FreeBSD code is completely untested. Use at your own risk. Feedback is welcome.
package tfo

import (
	"context"
	"errors"
	"net"
	"time"
)

var (
	ErrPlatformUnsupported = errors.New("tfo-go does not support TCP Fast Open on this platform")
	errMissingAddress      = errors.New("missing address")
)

// ListenConfig wraps Go's net.ListenConfig along with an option that allows you to disable TFO.
type ListenConfig struct {
	net.ListenConfig

	// DisableTFO controls whether TCP Fast Open is disabled on this instance of TFOListenConfig.
	// TCP Fast Open is enabled by default on TFOListenConfig.
	// Set to true to disable TFO and make TFOListenConfig behave exactly the same as net.ListenConfig.
	DisableTFO bool
}

// Listen announces on the local network address.
//
// See func Listen for a description of the network and address
// parameters.
//
// This function enables TFO whenever possible, unless ListenConfig.DisableTFO is set to true.
func (lc *ListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if lc.DisableTFO || network != "tcp" && network != "tcp4" && network != "tcp6" {
		return lc.ListenConfig.Listen(ctx, network, address)
	}
	return lc.listenTFO(ctx, network, address) // tfo_darwin.go, tfo_notdarwin.go
}

// ListenContext is a convenience function that allows you to specify a context within a single listen call.
//
// This function enables TFO whenever possible.
func ListenContext(ctx context.Context, network, address string) (net.Listener, error) {
	var lc ListenConfig
	return lc.Listen(ctx, network, address)
}

// Listen announces on the local network address.
//
// The network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
//
// For TCP networks, if the host in the address parameter is empty or
// a literal unspecified IP address, Listen listens on all available
// unicast and anycast IP addresses of the local system.
// To only use IPv4, use network "tcp4".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The Addr method of Listener can be used to discover the chosen
// port.
//
// See func Dial for a description of the network and address
// parameters.
//
// Listen uses context.Background internally; to specify the context, use
// ListenConfig.Listen.
//
// This function enables TFO whenever possible.
func Listen(network, address string) (net.Listener, error) {
	return ListenContext(context.Background(), network, address)
}

// ListenTCP acts like Listen for TCP networks.
//
// The network must be a TCP network name; see func Dial for details.
//
// If the IP field of laddr is nil or an unspecified IP address,
// ListenTCP listens on all available unicast and anycast IP addresses
// of the local system.
// If the Port field of laddr is 0, a port number is automatically
// chosen.
//
// This function enables TFO whenever possible.
func ListenTCP(network string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, &net.OpError{Op: "listen", Net: network, Source: nil, Addr: opAddr(laddr), Err: net.UnknownNetworkError(network)}
	}
	var address string
	if laddr != nil {
		address = laddr.String()
	}
	var lc ListenConfig
	ln, err := lc.listenTFO(context.Background(), network, address) // tfo_darwin.go, tfo_notdarwin.go
	if err != nil && err != ErrPlatformUnsupported {
		return nil, err
	}
	return ln.(*net.TCPListener), err
}

// Dialer wraps Go's net.Dialer along with an option that allows you to disable TFO.
type Dialer struct {
	net.Dialer

	// DisableTFO controls whether TCP Fast Open is disabled on this instance of TFODialer.
	// TCP Fast Open is enabled by default on TFODialer.
	// Set to true to disable TFO and make TFODialer behave exactly the same as net.Dialer.
	DisableTFO bool
}

// DialContext connects to the address on the named network using
// the provided context.
//
// The provided Context must be non-nil. If the context expires before
// the connection is complete, an error is returned. Once successfully
// connected, any expiration of the context will not affect the
// connection.
//
// When using TCP, and the host in the address parameter resolves to multiple
// network addresses, any dial timeout (from d.Timeout or ctx) is spread
// over each consecutive dial, such that each is given an appropriate
// fraction of the time to connect.
// For example, if a host has 4 IP addresses and the timeout is 1 minute,
// the connect to each single address will be given 15 seconds to complete
// before trying the next one.
//
// See func Dial for a description of the network and address
// parameters.
//
// This function enables TFO whenever possible, unless Dialer.DisableTFO is set to true.
func (d *Dialer) DialContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	if d.DisableTFO || len(b) == 0 || network != "tcp" && network != "tcp4" && network != "tcp6" {
		return d.Dialer.DialContext(ctx, network, address)
	}
	return d.dialTFOContext(ctx, network, address, b) // tfo_linux.go, tfo_windows_bsd.go, tfo_fallback.go
}

// Dial connects to the address on the named network.
//
// See func Dial for a description of the network and address
// parameters.
//
// Dial uses context.Background internally; to specify the context, use
// DialContext.
//
// This function enables TFO whenever possible, unless Dialer.DisableTFO is set to true.
func (d *Dialer) Dial(network, address string, b []byte) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address, b)
}

// Dial connects to the address on the named network.
//
// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
// "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
// (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
// "unixpacket".
//
// For TCP and UDP networks, the address has the form "host:port".
// The host must be a literal IP address, or a host name that can be
// resolved to IP addresses.
// The port must be a literal port number or a service name.
// If the host is a literal IPv6 address it must be enclosed in square
// brackets, as in "[2001:db8::1]:80" or "[fe80::1%zone]:80".
// The zone specifies the scope of the literal IPv6 address as defined
// in RFC 4007.
// The functions JoinHostPort and SplitHostPort manipulate a pair of
// host and port in this form.
// When using TCP, and the host resolves to multiple IP addresses,
// Dial will try each IP address in order until one succeeds.
//
// Examples:
//
//	Dial("tcp", "golang.org:http")
//	Dial("tcp", "192.0.2.1:http")
//	Dial("tcp", "198.51.100.1:80")
//	Dial("udp", "[2001:db8::1]:domain")
//	Dial("udp", "[fe80::1%lo0]:53")
//	Dial("tcp", ":80")
//
// For IP networks, the network must be "ip", "ip4" or "ip6" followed
// by a colon and a literal protocol number or a protocol name, and
// the address has the form "host". The host must be a literal IP
// address or a literal IPv6 address with zone.
// It depends on each operating system how the operating system
// behaves with a non-well known protocol number such as "0" or "255".
//
// Examples:
//
//	Dial("ip4:1", "192.0.2.1")
//	Dial("ip6:ipv6-icmp", "2001:db8::1")
//	Dial("ip6:58", "fe80::1%lo0")
//
// For TCP, UDP and IP networks, if the host is empty or a literal
// unspecified IP address, as in ":80", "0.0.0.0:80" or "[::]:80" for
// TCP and UDP, "", "0.0.0.0" or "::" for IP, the local system is
// assumed.
//
// For Unix networks, the address must be a file system path.
//
// This function enables TFO whenever possible.
func Dial(network, address string, b []byte) (net.Conn, error) {
	var d Dialer
	return d.DialContext(context.Background(), network, address, b)
}

// DialTimeout acts like Dial but takes a timeout.
//
// The timeout includes name resolution, if required.
// When using TCP, and the host in the address parameter resolves to
// multiple IP addresses, the timeout is spread over each consecutive
// dial, such that each is given an appropriate fraction of the time
// to connect.
//
// See func Dial for a description of the network and address
// parameters.
//
// This function enables TFO whenever possible.
func DialTimeout(network, address string, timeout time.Duration, b []byte) (net.Conn, error) {
	var d Dialer
	d.Timeout = timeout
	return d.DialContext(context.Background(), network, address, b)
}

// DialTCP acts like Dial for TCP networks.
//
// The network must be a TCP network name; see func Dial for details.
//
// If laddr is nil, a local address is automatically chosen.
// If the IP field of raddr is nil or an unspecified IP address, the
// local system is assumed.
//
// This function enables TFO whenever possible.
func DialTCP(network string, laddr, raddr *net.TCPAddr, b []byte) (*net.TCPConn, error) {
	if len(b) == 0 {
		return net.DialTCP(network, laddr, raddr)
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: opAddr(raddr), Err: net.UnknownNetworkError(network)}
	}
	if raddr == nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: opAddr(laddr), Addr: nil, Err: errMissingAddress}
	}
	return dialTFO(network, laddr, raddr, b, nil) // tfo_linux.go, tfo_windows.go, tfo_darwin.go, tfo_fallback.go
}

func opAddr(a *net.TCPAddr) net.Addr {
	if a == nil {
		return nil
	}
	return a
}
