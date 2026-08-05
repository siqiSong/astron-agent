package artifact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrHostNotAllowed = errors.New("download host is not allowed")
	ErrPrivateAddress = errors.New("download address is private")
	ErrDownloadLimit  = errors.New("download exceeds size limit")
	ErrInvalidFile    = errors.New("download is not a supported spreadsheet")
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type FetchPolicy struct {
	AllowedHosts         []string
	Resolver             Resolver
	MaxBytes             int64
	AllowPrivate         bool
	RedirectAllowPrivate bool
}

type Fetched struct {
	Data   []byte
	Format string
	URL    string
}

func Fetch(ctx context.Context, rawURL string, policy FetchPolicy) (Fetched, error) {
	if policy.MaxBytes <= 0 {
		policy.MaxBytes = 50 << 20
	}
	if policy.Resolver == nil {
		policy.Resolver = net.DefaultResolver
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return Fetched{}, ErrHostNotAllowed
	}
	if err := validateTarget(ctx, parsed, policy, policy.AllowPrivate); err != nil {
		return Fetched{}, err
	}
	redirects := 0
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		redirects++
		if redirects > 3 {
			return errors.New("too many redirects")
		}
		return validateTarget(request.Context(), request.URL, policy, policy.RedirectAllowPrivate)
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Fetched{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return Fetched{}, unwrapURLValidation(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Fetched{}, fmt.Errorf("download status %d", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, policy.MaxBytes+1))
	if err != nil {
		return Fetched{}, err
	}
	if int64(len(content)) > policy.MaxBytes {
		return Fetched{}, ErrDownloadLimit
	}
	format := detectFetchedFormat(content)
	if format == "" {
		return Fetched{}, ErrInvalidFile
	}
	return Fetched{Data: content, Format: format, URL: response.Request.URL.String()}, nil
}

func validateTarget(ctx context.Context, target *url.URL, policy FetchPolicy, allowPrivate bool) error {
	host := strings.ToLower(target.Hostname())
	allowed := false
	for _, candidate := range policy.AllowedHosts {
		if host == strings.ToLower(strings.TrimSpace(candidate)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrHostNotAllowed
	}
	addresses, err := policy.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return ErrHostNotAllowed
	}
	if !allowPrivate {
		for _, address := range addresses {
			if isPrivateIP(address.IP) {
				return ErrPrivateAddress
			}
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

func unwrapURLValidation(err error) error {
	for _, target := range []error{ErrHostNotAllowed, ErrPrivateAddress} {
		if errors.Is(err, target) {
			return target
		}
	}
	return err
}

func detectFetchedFormat(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return "xls"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 3, 4}) {
		return "xlsx"
	}
	if len(data) > 0 && !bytes.Contains(data[:min(len(data), 512)], []byte{0}) && (bytes.Contains(data, []byte(",")) || bytes.Contains(data, []byte("\t"))) {
		return "csv"
	}
	return ""
}
