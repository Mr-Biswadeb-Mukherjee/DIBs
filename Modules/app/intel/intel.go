package intel

import (
	"context"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/intel/dns_intel"
)

//
// ==============================
// Public Types (EXPOSED)
// ==============================
//

type Domain struct {
	Name string
}

type Record struct {
	Domain    string
	A         []string
	AAAA      []string
	CNAME     []string
	NS        []string
	MX        []string
	TXT       []string
	Providers []string
}

//
// ==============================
// Service
// ==============================
//

type DNSIntelService struct {
	processor *dns_intel.Processor
}

func NewDNSIntelService(
	resolver dns_intel.Resolver,
	cache dns_intel.Cache,
	workers int,
	timeout time.Duration,
) *DNSIntelService {

	return &DNSIntelService{
		processor: dns_intel.NewProcessor(resolver, cache, workers, timeout),
	}
}

//
// ==============================
// Public API
// ==============================
//

func (s *DNSIntelService) Run(
	ctx context.Context,
	domains []Domain,
) ([]Record, error) {

	if len(domains) == 0 {
		return nil, nil
	}

	// convert → dns_intel types
	in := make([]dns_intel.DomainRecord, 0, len(domains))
	for _, d := range domains {
		in = append(in, dns_intel.DomainRecord{
			Domain: d.Name,
		})
	}

	out, err := s.processor.Process(ctx, in)
	if err != nil {
		return nil, err
	}

	// convert back → public types
	res := make([]Record, 0, len(out))
	for _, r := range out {
		res = append(res, Record{
			Domain:    r.Domain,
			A:         r.A,
			AAAA:      r.AAAA,
			CNAME:     r.CNAME,
			NS:        r.NS,
			MX:        r.MX,
			TXT:       r.TXT,
			Providers: r.Providers,
		})
	}

	return res, nil
}