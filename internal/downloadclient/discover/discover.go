package discover

import (
	"context"
	"strings"
	"sync"
	"time"

)

const (
	defaultDiscoverTimeout = 25 * time.Second
	fingerprintWorkers     = 16
)

// Options controls discovery behavior.
type Options struct {
	// SkipTrueNASRange skips the 30000–32000 TCP sweep (useful in unit tests).
	SkipTrueNASRange bool
	// SSDPWait overrides the SSDP listen window; zero uses the default.
	SSDPWait time.Duration
	// Existing clients are excluded from suggestions (URL / IP:port / impl+port).
	Existing []ExistingClient
	// ExtraSubnets are optional CIDR overrides from AppConfig; normally LAN is
	// learned from the router (IGD) and host routes.
	ExtraSubnets []string
	// HostProcNet is an optional path to a bind-mounted host /proc/net directory.
	HostProcNet string
}

// Discover finds nearby download clients via SSDP/IGD/host routes + TCP/HTTP fingerprints.
func Discover(ctx context.Context, opts Options) ([]Result, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultDiscoverTimeout)
		defer cancel()
	}

	extra := opts.ExtraSubnets

	local := localHostHints()
	ssdp := discoverSSDP(ctx, opts.SSDPWait)
	routerNets := discoverRouterLANNetworks(ctx, ssdp.LocationURLs)
	hostNets := hostLANNetworksFromProcRoute(opts.HostProcNet)
	subnet := subnetHostHints(append(append([]lanNetwork{}, routerNets...), hostNets...), extra)
	hosts := mergeHostHints(local, ssdp.Hosts, subnet)

	open := scanOpenPorts(ctx, hosts, !opts.SkipTrueNASRange)
	if len(open) == 0 {
		return nil, nil
	}

	type hit struct {
		res Result
		ok  bool
	}
	jobs := make(chan openPort, len(open))
	outCh := make(chan hit, len(open))

	var wg sync.WaitGroup
	workers := fingerprintWorkers
	if workers > len(open) {
		workers = len(open)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for op := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				res, ok := fingerprintOpenPort(ctx, op)
				outCh <- hit{res: res, ok: ok}
			}
		}()
	}
	for _, op := range open {
		jobs <- op
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(outCh)
	}()

	var raw []Result
	seenURL := map[string]struct{}{}
	for h := range outCh {
		if !h.ok {
			continue
		}
		urlKey := strings.ToLower(h.res.BaseURL)
		if _, ok := seenURL[urlKey]; ok {
			continue
		}
		seenURL[urlKey] = struct{}{}
		raw = append(raw, h.res)
	}

	out := dedupeResults(raw)
	out = dropGatewayWhenComposeExists(out)
	out = filterExisting(out, opts.Existing)
	return out, nil
}
