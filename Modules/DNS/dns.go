package DNS

import (
    "context"
    "errors"
    "math/rand"
    "sync"
    "time"

    "github.com/miekg/dns"
)

var upstreamServers = []string{
    "8.8.8.8:53",
    "1.1.1.1:53",
}

var ErrNoUpstream = errors.New("no upstream DNS servers responded")

// ---------------------------------------------------------------
// Client pool (SO_REUSEPORT-like effect for massively parallel DNS)
// ---------------------------------------------------------------
var (
    clientPool []*dns.Client
    poolOnce   sync.Once
)

func initClientPool() {
    poolOnce.Do(func() {
        // Create a pool of UDP clients, each has its own socket
        clientPool = make([]*dns.Client, 32)
        for i := 0; i < len(clientPool); i++ {
            clientPool[i] = &dns.Client{
                Net:            "udp",
                Timeout:        200 * time.Millisecond,
                UDPSize:        4096,
                SingleInflight: false,
            }
        }
    })
}

func getClient() *dns.Client {
    return clientPool[rand.Intn(len(clientPool))]
}

// ---------------------------------------------------------------
// Context-aware exchange (fast, non-blocking, reusable client)
// ---------------------------------------------------------------
func exchangeCtx(ctx context.Context, client *dns.Client, msg *dns.Msg, upstream string) (*dns.Msg, error) {
    respCh := make(chan *dns.Msg, 1)
    errCh := make(chan error, 1)

    go func() {
        resp, _, err := client.Exchange(msg, upstream)
        if err != nil {
            errCh <- err
            return
        }
        respCh <- resp
    }()

    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case resp := <-respCh:
        return resp, nil
    case err := <-errCh:
        return nil, err
    }
}

// ---------------------------------------------------------------
// Parallel upstream resolver with early-exit win
// ---------------------------------------------------------------
func forwardQueryCtx(ctx context.Context, r *dns.Msg) (*dns.Msg, error) {
    if len(upstreamServers) == 0 {
        return nil, ErrNoUpstream
    }

    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    initClientPool()                     // ensure pool exists
    client := getClient()                // random client → random socket
    shuffled := shuffleServers()         // randomizes upstream order
    out := make(chan *dns.Msg, 1)
    errOut := make(chan error, len(shuffled))

    // spawn upstream queries
    for _, up := range shuffled {
        u := up
        go func() {
            resp, err := exchangeCtx(ctx, client, r, u)
            if err != nil {
                errOut <- err
                return
            }
            // Only accept a DNS response with an Answer section
            if resp != nil && len(resp.Answer) > 0 {
                select {
                case out <- resp:
                    cancel()
                default:
                }
                return
            }
            errOut <- ErrNoUpstream
        }()
    }

    // wait for first valid response or all failures
    failures := 0
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()

        case resp := <-out:
            return resp, nil

        case <-errOut:
            failures++
            if failures == len(shuffled) {
                return nil, ErrNoUpstream
            }
        }
    }
}

func shuffleServers() []string {
    // simple fast shuffle
    out := make([]string, len(upstreamServers))
    copy(out, upstreamServers)

    for i := range out {
        j := rand.Intn(i + 1)
        out[i], out[j] = out[j], out[i]
    }
    return out
}

// ---------------------------------------------------------------
// Public API (A + AAAA with fallback)
// ---------------------------------------------------------------
func Resolve(ctx context.Context, domain string) bool {
    fqdn := dns.Fqdn(domain)

    // A record
    msg := new(dns.Msg)
    msg.SetQuestion(fqdn, dns.TypeA)
    msg.RecursionDesired = true

    if resp, err := forwardQueryCtx(ctx, msg); err == nil && resp != nil {
        return true
    }

    // AAAA fallback
    msg6 := new(dns.Msg)
    msg6.SetQuestion(fqdn, dns.TypeAAAA)
    msg6.RecursionDesired = true

    if resp6, err := forwardQueryCtx(ctx, msg6); err == nil && resp6 != nil {
        return true
    }

    return false
}
