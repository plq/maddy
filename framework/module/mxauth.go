package module

import (
	"context"
	"crypto/tls"
)

const (
	AuthDisabled     = "off"
	AuthMTASTS       = "mtasts"
	AuthDNSSEC       = "dnssec"
	AuthCommonDomain = "common_domain"
)

type (
	TLSLevel int
	MXLevel  int
)

const (
	TLSNone TLSLevel = iota
	TLSEncrypted
	TLSAuthenticated

	MXNone MXLevel = iota
	MX_MTASTS
	MX_DNSSEC
)

func (l TLSLevel) String() string {
	switch l {
	case TLSNone:
		return "none"
	case TLSEncrypted:
		return "encrypted"
	case TLSAuthenticated:
		return "authenticated"
	}
	return "???"
}

func (l MXLevel) String() string {
	switch l {
	case MXNone:
		return "none"
	case MX_MTASTS:
		return "mtasts"
	case MX_DNSSEC:
		return "dnssec"
	}
	return "???"
}

type (
	// MXAuthPolicy is an object that provides security check for outbound connections.
	// It can do one of the following:
	//
	// - Check effective TLS level or MX level against some configured or
	// discovered value.
	// E.g. local policy.
	//
	// - Raise the security level if certain condition about used MX or
	// connection is met.
	// E.g. DANE MXAuthPolicy raises TLS level to Authenticated if a matching
	// TLSA record is discovered.
	//
	// - Reject the connection if certain condition about used MX or
	// connection is _not_ met.
	// E.g. An enforced MTA-STS MXAuthPolicy rejects MX records not matching it.
	//
	// It is not recommended to mix different types of behavior described above
	// in the same implementation.
	// Specifically, the first type is used mostly for local policies and is not
	// really practical.
	//
	// Modules implementing this interface should be registered with "mx_auth."
	// prefix in name.
	MXAuthPolicy interface {
		Start(*MsgMetadata) DeliveryMXAuthPolicy

		// Weight is an integer in range 0-1000 that represents relative
		// ordering of policy application.
		Weight() int
	}

	// DeliveryMXAuthPolicy is an interface of per-delivery object that estabilishes
	// and verifies required and effective security for MX records and TLS
	// connections.
	DeliveryMXAuthPolicy interface {
		// PrepareDomain is called before DNS MX lookup and may asynchronously
		// start additional lookups necessary for policy application in CheckMX
		// or CheckConn.
		//
		// If there any errors - they should be deferred to the CheckMX or
		// CheckConn call.
		PrepareDomain(ctx context.Context, domain string)

		// PrepareDomain is called before connection and may asynchronously
		// start additional lookups necessary for policy application in
		// CheckConn.
		//
		// If there any errors - they should be deferred to the CheckConn
		// call.
		PrepareConn(ctx context.Context, mx string)

		// CheckMX is called to check whether the policy permits to use a MX.
		//
		// mxLevel contains the MX security level estabilished by checks
		// executed before.
		//
		// domain is passed to the CheckMX to allow simpler implementation
		// of stateless policy objects.
		//
		// dnssec is true if the MX lookup was performed using DNSSEC-enabled
		// resolver and the zone is signed and its signature is valid.
		CheckMX(ctx context.Context, mxLevel MXLevel, domain, mx string, dnssec bool) (MXLevel, error)

		// CheckConn is called to check whether the policy permits to use this
		// connection.
		//
		// tlsLevel and mxLevel contain the TLS security level estabilished by
		// checks executed before.
		//
		// domain is passed to the CheckConn to allow simpler implementation
		// of stateless policy objects.
		//
		// If tlsState.HandshakeCompleted is false, TLS is not used. If
		// tlsState.VerifiedChains is nil, InsecureSkipVerify was used (no
		// ServerName or PKI check was done).
		CheckConn(ctx context.Context, mxLevel MXLevel, tlsLevel TLSLevel, domain, mx string, tlsState tls.ConnectionState) (TLSLevel, error)

		// Reset cleans the internal object state for use with another message.
		// newMsg may be nil if object is not needed anymore.
		Reset(newMsg *MsgMetadata)
	}
)
