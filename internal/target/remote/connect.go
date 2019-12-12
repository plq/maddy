package remote

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"runtime/trace"
	"sort"
	"strings"

	"github.com/foxcpp/maddy/internal/config"
	"github.com/foxcpp/maddy/internal/dns"
	"github.com/foxcpp/maddy/internal/exterrors"
	"github.com/foxcpp/maddy/internal/future"
	"github.com/foxcpp/maddy/internal/mtasts"
	"github.com/foxcpp/maddy/internal/smtpconn"
	"golang.org/x/net/publicsuffix"
)

type mxConn struct {
	*smtpconn.C

	// Domain this MX belongs to.
	domain string

	// Future value holding the MTA-STS fetch results for this domain.
	// May be nil if MTA-STS is disabled.
	stsFut *future.Future

	// Future value holding the TLSA lookup results for this MX.
	// May be nil if DANE is disabled.
	tlsaFut *future.Future

	// The MX record is DNSSEC-signed and was verified by the used resolver.
	dnssecOk bool
}

func (rd *remoteDelivery) checkPolicies(ctx context.Context, mx string, didTLS bool, conn mxConn) error {
	var (
		authenticated bool
		requireTLS    bool
	)

	// Implicit MX is always authenticated (and explicit one with the same
	// hostname too).
	if dns.Equal(mx, conn.domain) {
		authenticated = true
	}

	// Apply recipient MTA-STS policy.
	if conn.stsFut != nil {
		stsPolicy, err := conn.stsFut.GetContext(ctx)
		if err == nil {
			stsPolicy := stsPolicy.(*mtasts.Policy)

			// Respect the policy: require TLS if it exists, skip non-matching MXs.
			// Any policy (even None) is enough to mark MX as 'authenticated'.
			requireTLS = stsPolicy.Mode == mtasts.ModeEnforce
			if requireTLS {
				rd.Log.DebugMsg("TLS required by MTA-STS", "mx", mx, "domain", conn.domain)
			}
			if stsPolicy.Match(mx) {
				rd.Log.DebugMsg("authenticated MX using MTA-STS", "mx", mx)
				authenticated = true
			} else if stsPolicy.Mode == mtasts.ModeEnforce {
				rd.Log.Msg("skipping MX not matching MTA-STS", "mx", mx, "domain", conn.domain)
				return &exterrors.SMTPError{
					Code:         550,
					EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
					Message:      "Failed to estabilish the MX record authenticity (MTA-STS)",
				}
			}
		} else {
			rd.Log.DebugMsg("Policy fetch error, ignoring", "mx", mx, "domain", conn.domain, "err", err)
		}
	}

	// Mark MX as 'authenticated' if DNS zone is signed.
	if _, do := rd.rt.mxAuth[AuthDNSSEC]; do && conn.dnssecOk {
		rd.Log.DebugMsg("authenticated MX using DNSSEC", "mx", mx, "domain", conn.domain)
		authenticated = true
	}

	// Mark MX as 'authenticated' if they have the same eTLD+1 domain part.
	if _, do := rd.rt.mxAuth[AuthCommonDomain]; do && commonDomainCheck(conn.domain, mx) {
		rd.Log.DebugMsg("authenticated MX using common domain rule", "mx", mx, "domain", conn.domain)
		authenticated = true
	}

	// Apply local policy.
	if rd.rt.requireTLS {
		rd.Log.DebugMsg("TLS required by local policy", "mx", mx, "domain", conn.domain)
		requireTLS = true
	}

	// Make decision based on the policy and connection state.
	if rd.rt.requireMXAuth && !authenticated {
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 0},
			Message:      fmt.Sprintf("Failed to estabilish the MX record (%s) authenticity", mx),
		}
	}
	if requireTLS && !didTLS {
		return &exterrors.SMTPError{
			Code:         550,
			EnhancedCode: exterrors.EnhancedCode{5, 7, 1},
			Message:      fmt.Sprintf("TLS is required but unsupported or failed (mx = %s)", mx),
		}
	}

	// All green.
	return nil
}

func (rd *remoteDelivery) connectionForDomain(ctx context.Context, domain string) (*smtpconn.C, error) {
	domain = strings.ToLower(domain)

	if c, ok := rd.connections[domain]; ok {
		return c.C, nil
	}

	conn := mxConn{
		C:      smtpconn.New(),
		domain: domain,
	}

	conn.Dialer = rd.rt.dialer
	conn.TLSConfig = rd.rt.tlsConfig
	conn.Log = rd.Log
	conn.Hostname = rd.rt.hostname
	conn.AddrInSMTPMsg = true

	if _, do := rd.rt.mxAuth[AuthMTASTS]; do {
		conn.stsFut = future.New()
		if rd.rt.mtastsGet != nil {
			go func() {
				conn.stsFut.Set(rd.rt.mtastsGet(ctx, domain))
			}()
		} else {
			go func() {
				conn.stsFut.Set(rd.rt.mtastsCache.Get(ctx, domain))
			}()
		}
	}

	region := trace.StartRegion(ctx, "remote/LookupMX")
	dnssecOk, records, err := rd.lookupMX(ctx, domain)
	region.End()
	if err != nil {
		return nil, err
	}
	conn.dnssecOk = dnssecOk

	var (
		lastErr    error
		daneCancel func()
	)
	region = trace.StartRegion(ctx, "remote/Connect+TLS")
	for _, record := range records {
		if daneCancel != nil {
			daneCancel()
			daneCancel = nil
		}

		if record.Host == "." {
			return nil, &exterrors.SMTPError{
				Code:         556,
				EnhancedCode: exterrors.EnhancedCode{5, 1, 10},
				Message:      "Domain does not accept email (null MX)",
			}
		}

		if rd.rt.dane {
			var daneCtx context.Context
			daneCtx, daneCancel = context.WithCancel(ctx)
			conn.tlsaFut = future.New()
			go func() {
				conn.tlsaFut.Set(rd.lookupTLSA(daneCtx, record.Host))
			}()
		}

		rd.Log.DebugMsg("trying", "mx", record.Host, "domain", domain)
		didTLS, err := conn.Connect(ctx, config.Endpoint{
			Host: record.Host,
			Port: smtpPort,
		}, true)
		authErr := rd.checkPolicies(ctx, record.Host, didTLS, conn)

		if err != nil {
			lastErr = err

			// If there was a TLS error and MX auth does not seem to complain
			// about plaintext - reconnect without TLS.
			if _, ok := err.(smtpconn.TLSError); ok && authErr == nil {
				// Check if DANE is here and prevents our fallback.
				if err := rd.verifyDANE(ctx, conn, tls.ConnectionState{}); err != nil {
					lastErr = err
					continue
				}

				rd.Log.Error("TLS error, falling back to plaintext", err,
					"mx", record.Host, "domain", domain)

				_, err := conn.Connect(ctx, config.Endpoint{
					Host: record.Host,
					Port: smtpPort,
				}, false)
				if err != nil {
					// That's odd, but whatever.
					continue
				}
			} else {
				continue
			}
		}

		if authErr != nil {
			conn.Close()
			lastErr = authErr
			continue
		}

		tlsState, _ := conn.Client().TLSConnectionState()
		if err := rd.verifyDANE(ctx, conn, tlsState); err != nil {
			conn.Close()
			lastErr = err
			continue
		}

		break
	}
	region.End()

	// Cancel it to make staticcheck happy. verifyDANE call in loop above
	// will wait for it to complete anyway.
	if daneCancel != nil {
		daneCancel()
	}

	// Stil not connected? Bail out.
	if conn.Client() == nil {
		return nil, &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(err, 451, 550),
			EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 4, 0}),
			Message:      "No usable MXs, last err: " + lastErr.Error(),
			TargetName:   "remote",
			Err:          lastErr,
			Misc: map[string]interface{}{
				"domain": domain,
			},
		}
	}

	rd.Log.DebugMsg("connected", "mx", conn.ServerName(), "domain", domain)

	if err := conn.Mail(ctx, rd.mailFrom, rd.msgMeta.SMTPOpts); err != nil {
		conn.Close()
		return nil, err
	}

	rd.connections[domain] = conn
	return conn.C, nil
}

func commonDomainCheck(domain string, mx string) bool {
	domainPart, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return false
	}
	mxPart, err := publicsuffix.EffectiveTLDPlusOne(strings.TrimSuffix(mx, "."))
	if err != nil {
		return false
	}

	return domainPart == mxPart
}

func (rd *remoteDelivery) lookupMX(ctx context.Context, domain string) (dnssecOk bool, records []*net.MX, err error) {
	if _, use := rd.rt.mxAuth[AuthDNSSEC]; use {
		if rd.rt.extResolver == nil {
			return false, nil, errors.New("remote: can't do DNSSEC verification without security-aware resolver")
		}
		dnssecOk, records, err = rd.rt.extResolver.AuthLookupMX(context.Background(), domain)
	} else {
		records, err = rd.rt.resolver.LookupMX(ctx, domain)
	}
	if err != nil {
		reason, misc := exterrors.UnwrapDNSErr(err)
		return false, nil, &exterrors.SMTPError{
			Code:         exterrors.SMTPCode(err, 451, 554),
			EnhancedCode: exterrors.SMTPEnchCode(err, exterrors.EnhancedCode{0, 4, 4}),
			Message:      "MX lookup error",
			TargetName:   "remote",
			Reason:       reason,
			Err:          err,
			Misc:         misc,
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})

	// Fallback to A/AAA RR when no MX records are present as
	// required by RFC 5321 Section 5.1.
	if len(records) == 0 {
		records = append(records, &net.MX{
			Host: domain,
			Pref: 0,
		})
	}

	return dnssecOk, records, err
}