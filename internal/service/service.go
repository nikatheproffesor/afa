// Package service measures DNS, TCP and TLS connectivity to a single host.
package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"afa/internal/diagnostic"
)

const (
	dnsTimeout    = 3 * time.Second
	connectBudget = 4 * time.Second
	fallbackDelay = 250 * time.Millisecond

	statusOK      = "ok"
	statusFailed  = "failed"
	statusSkipped = "skipped"
)

type Result struct {
	Host            string          `json:"host"`
	Port            string          `json:"port"`
	DNS             string          `json:"dns"`
	TCP             string          `json:"tcp"`
	TLS             string          `json:"tls"`
	DNSMillis       *int64          `json:"dns_millis,omitempty"`
	TCPMillis       *int64          `json:"tcp_millis,omitempty"`
	TLSMillis       *int64          `json:"tls_millis,omitempty"`
	AddressFamilies []string        `json:"address_families,omitempty"`
	ConnectedFamily string          `json:"connected_family,omitempty"`
	ErrorCode       diagnostic.Code `json:"error_code,omitempty"`
	ErrorStage      string          `json:"error_stage,omitempty"`
}

type resolver func(context.Context, string) ([]net.IPAddr, error)
type dialer func(context.Context, string, string) (net.Conn, error)

// Check resolves the URL's hostname and checks TLS on its port. It performs no
// HTTP request. Resolution and connection failures are reflected in the result.
func Check(ctx context.Context, rawURL string) (Result, error) {
	dial := (&net.Dialer{}).DialContext
	return check(ctx, rawURL, net.DefaultResolver.LookupIPAddr, dial, nil)
}

func check(ctx context.Context, rawURL string, lookup resolver, dial dialer, roots *x509.CertPool) (Result, error) {
	host, port, err := parseTarget(rawURL)
	if err != nil {
		return Result{}, err
	}
	result := Result{Host: host, Port: port, DNS: statusSkipped, TCP: statusSkipped, TLS: statusSkipped}
	if err := ctx.Err(); err != nil {
		result.ErrorCode = classify(err, diagnostic.DNSFailure)
		return result, nil
	}

	dnsCtx, cancelDNS := context.WithTimeout(ctx, dnsTimeout)
	started := time.Now()
	addresses, err := lookup(dnsCtx, host)
	dnsMillis := time.Since(started).Milliseconds()
	result.DNSMillis = &dnsMillis
	if dnsCtx.Err() != nil {
		err = dnsCtx.Err()
	}
	cancelDNS()
	families := partitionAddresses(addresses)
	if err != nil || len(families) == 0 {
		result.DNS = statusFailed
		result.ErrorCode = classify(err, diagnostic.DNSFailure)
		result.ErrorStage = "dns"
		return result, nil
	}
	result.DNS = statusOK
	for _, family := range families {
		result.AddressFamilies = append(result.AddressFamilies, family.name)
	}

	connectCtx, cancelConnect := context.WithTimeout(ctx, connectBudget)
	defer cancelConnect()
	tlsConfig := &tls.Config{
		ServerName: strings.TrimSuffix(host, "."),
		MinVersion: tls.VersionTLS12,
		RootCAs:    roots,
	}
	observation := probeFamilies(connectCtx, dial, port, tlsConfig, families)
	result.TCPMillis = observation.tcpMillis
	result.ErrorCode = observation.failure
	if observation.tcpMillis != nil {
		result.TCP = statusFailed
	}
	if observation.tcpConnected {
		result.TCP = statusOK
		result.ConnectedFamily = observation.family
		result.TLS = statusFailed
		result.TLSMillis = &observation.tlsMillis
	}
	if observation.failure == "" {
		result.TLS = statusOK
		return result, nil
	}
	result.ErrorStage = "tcp"
	if observation.tcpConnected {
		result.ErrorStage = "tls"
	}
	if ctx.Err() == context.Canceled || (connectCtx.Err() != nil && result.ErrorCode != diagnostic.TLSCertificate) {
		result.ErrorCode = classify(connectCtx.Err(), result.ErrorCode)
	}
	return result, nil
}

type addressFamily struct {
	name      string
	addresses []net.IPAddr
}

func partitionAddresses(addresses []net.IPAddr) []addressFamily {
	v4 := addressFamily{name: "ipv4"}
	v6 := addressFamily{name: "ipv6"}
	var preferred string
	for _, address := range addresses {
		var family *addressFamily
		switch {
		case address.IP.To4() != nil:
			family = &v4
		case address.IP.To16() != nil:
			family = &v6
		default:
			continue
		}
		family.addresses = append(family.addresses, address)
		if preferred == "" {
			preferred = family.name
		}
	}
	if len(v4.addresses) == 0 && len(v6.addresses) == 0 {
		return nil
	}
	if len(v4.addresses) == 0 {
		return []addressFamily{v6}
	}
	if len(v6.addresses) == 0 {
		return []addressFamily{v4}
	}
	if preferred == v6.name {
		return []addressFamily{v6, v4}
	}
	return []addressFamily{v4, v6}
}

type attempt struct {
	tcpMillis    *int64
	tcpConnected bool
	tlsMillis    int64
	family       string
	failure      diagnostic.Code
}

func probeFamilies(ctx context.Context, dial dialer, port string, config *tls.Config, families []addressFamily) attempt {
	if len(families) == 1 {
		return probeFamily(ctx, dial, port, config, families[0])
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	observations := make(chan attempt, 2)
	probe := func(family addressFamily) { observations <- probeFamily(ctx, dial, port, config, family) }
	go probe(families[0])
	timer := time.NewTimer(fallbackDelay)
	defer timer.Stop()
	fallback := timer.C
	remaining := 1
	startFallback := func() {
		fallback = nil
		if ctx.Err() == nil {
			remaining++
			go probe(families[1])
		}
	}
	var best attempt
	for remaining > 0 {
		select {
		case <-fallback:
			startFallback()
		case result := <-observations:
			remaining--
			if result.failure == "" {
				cancel()
				for remaining > 0 {
					<-observations
					remaining--
				}
				return result
			}
			best = betterAttempt(best, result)
			if fallback != nil {
				startFallback()
			}
		}
	}
	return best
}

func probeFamily(ctx context.Context, dial dialer, port string, config *tls.Config, family addressFamily) attempt {
	best := attempt{failure: diagnostic.TCPFailure}
	for i, address := range family.addresses {
		if err := ctx.Err(); err != nil {
			if best.tcpMillis == nil {
				best.failure = classify(err, best.failure)
			}
			break
		}
		attemptCtx, cancel := ctx, func() {}
		if deadline, ok := ctx.Deadline(); ok && i+1 < len(family.addresses) {
			budget := time.Until(deadline) / time.Duration(len(family.addresses)-i)
			attemptCtx, cancel = context.WithTimeout(ctx, budget)
		}
		result := probeAddress(attemptCtx, dial, net.JoinHostPort(address.String(), port), config, family.name)
		cancel()
		if result.failure == "" {
			return result
		}
		best = betterAttempt(best, result)
	}
	return best
}

func probeAddress(ctx context.Context, dial dialer, addr string, config *tls.Config, family string) attempt {
	started := time.Now()
	conn, err := dial(ctx, "tcp", addr)
	tcpMillis := time.Since(started).Milliseconds()
	result := attempt{tcpMillis: &tcpMillis, family: family}
	if err != nil {
		result.failure = classify(preferContextError(ctx, err), diagnostic.TCPFailure)
		return result
	}
	defer conn.Close()
	result.tcpConnected = true
	started = time.Now()
	err = tls.Client(conn, config).HandshakeContext(ctx)
	result.tlsMillis = time.Since(started).Milliseconds()
	if err != nil {
		result.failure = classify(preferContextError(ctx, err), diagnostic.TLSFailure)
	}
	return result
}

func betterAttempt(best, current attempt) attempt {
	if best.tcpMillis == nil || current.failure == diagnostic.TLSCertificate ||
		(current.tcpConnected && !best.tcpConnected) {
		return current
	}
	return best
}

func preferContextError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func classify(err error, fallback diagnostic.Code) diagnostic.Code {
	var cert *tls.CertificateVerificationError
	switch {
	case err == nil:
		return fallback
	case errors.As(err, &cert):
		return diagnostic.TLSCertificate
	default:
		return diagnostic.CodeOf(err, fallback)
	}
}

func parseTarget(rawURL string) (host, port string, err error) {
	invalidURL := errors.New("alan adı içeren geçerli bir HTTPS adresi girin")
	if strings.ContainsFunc(rawURL, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return "", "", invalidURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		strings.ContainsAny(parsed.Host, "[]") || strings.HasSuffix(parsed.Host, ":") {
		return "", "", invalidURL
	}

	host = parsed.Hostname()
	if !validDomain(host) {
		return "", "", invalidURL
	}
	port = "443"
	if specified := parsed.Port(); specified != "" {
		number, err := strconv.Atoi(specified)
		if err != nil || number < 1 || number > 65535 {
			return "", "", errors.New("port numarası 1 ile 65535 arasında olmalı")
		}
		port = strconv.Itoa(number)
	}
	return strings.ToLower(host), port, nil
}

func validDomain(host string) bool {
	name := strings.TrimSuffix(host, ".")
	if len(name) > 253 || net.ParseIP(name) != nil {
		return false
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !isASCIILetter(r) && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return strings.ContainsFunc(labels[len(labels)-1], isASCIILetter)
}

func isASCIILetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'
}
