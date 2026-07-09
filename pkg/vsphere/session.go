package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"k8s.io/klog/v2"
)

// Session wraps a govmomi client connection along with a pre-configured Finder
// and a REST-based tag manager for a specific datacenter.
type Session struct {
	// Client is the govmomi SOAP client.
	Client *govmomi.Client
	// Finder is a govmomi inventory finder with the datacenter already set.
	Finder *find.Finder
	// TagManager is a REST-based tag manager for vSphere tagging operations.
	TagManager *tags.Manager
	// Datacenter is the name of the datacenter this session is scoped to.
	Datacenter string

	restClient *rest.Client
}

// Params holds the parameters needed to establish a vSphere session.
type Params struct {
	// Server is the vCenter hostname or IP address.
	Server string
	// Datacenter is the name of the target datacenter.
	Datacenter string
	// Username is the vCenter login username.
	Username string
	// Password is the vCenter login password.
	Password string
	// Insecure controls whether TLS certificate verification is skipped.
	Insecure bool
}

var (
	sessionMu    sync.Mutex
	sessionCache = make(map[string]*Session)
)

// SanitizeServer strips any URL scheme and trailing path from a server
// value so that it contains only a hostname or host:port suitable for
// use in url.URL.Host.
func SanitizeServer(server string) string {
	server = strings.TrimSpace(server)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(server), prefix) {
			server = server[len(prefix):]
			break
		}
	}
	if idx := strings.Index(server, "/"); idx >= 0 {
		server = server[:idx]
	}
	return server
}

func cacheKey(p Params) string {
	return fmt.Sprintf("%s#%s#%s", p.Server, p.Datacenter, p.Username)
}

// NewSession creates a new vSphere session by connecting to the vCenter server
// specified in the given Params.
func NewSession(ctx context.Context, p Params) (*Session, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating new vSphere session", "server", p.Server, "datacenter", p.Datacenter)

	host := SanitizeServer(p.Server)
	u := &url.URL{
		Scheme: "https",
		Host:   host,
		Path:   vim25.Path,
	}
	u.User = url.UserPassword(p.Username, p.Password)

	soapClient := soap.NewClient(u, p.Insecure)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("creating vim25 client for %s: %w", host, err)
	}

	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	if err := client.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("logging in to %s: %w", p.Server, err)
	}

	finder := find.NewFinder(vimClient, true)

	dc, err := finder.Datacenter(ctx, p.Datacenter)
	if err != nil {
		return nil, fmt.Errorf("finding datacenter %q: %w", p.Datacenter, err)
	}
	finder.SetDatacenter(dc)

	rc := rest.NewClient(vimClient)
	if err := rc.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("REST login to %s: %w", p.Server, err)
	}

	tagMgr := tags.NewManager(rc)

	return &Session{
		Client:     client,
		Finder:     finder,
		TagManager: tagMgr,
		Datacenter: p.Datacenter,
		restClient: rc,
	}, nil
}

// Close logs out the SOAP and REST sessions.
func (s *Session) Close(ctx context.Context) {
	log := klog.FromContext(ctx)
	if s.restClient != nil {
		if err := s.restClient.Logout(ctx); err != nil {
			log.V(2).Info("REST logout error", "err", err)
		}
	}
	if s.Client != nil {
		if err := s.Client.Logout(ctx); err != nil {
			log.V(2).Info("SOAP logout error", "err", err)
		}
	}
}

// GetOrCreate returns an existing cached session for the given Params or creates
// a new one. Sessions are cached by server, datacenter, and username.
func GetOrCreate(ctx context.Context, p Params) (*Session, error) {
	key := cacheKey(p)

	sessionMu.Lock()
	defer sessionMu.Unlock()

	if s, ok := sessionCache[key]; ok {
		return s, nil
	}

	s, err := NewSession(ctx, p)
	if err != nil {
		return nil, err
	}

	sessionCache[key] = s
	return s, nil
}

// EnsureVMFolder creates /<datacenter>/vm/<folderName> if it doesn't exist.
func EnsureVMFolder(ctx context.Context, s *Session, folderName string) error {
	log := klog.FromContext(ctx)

	dc, err := s.Finder.Datacenter(ctx, s.Datacenter)
	if err != nil {
		return fmt.Errorf("finding datacenter %q: %w", s.Datacenter, err)
	}
	folders, err := dc.Folders(ctx)
	if err != nil {
		return fmt.Errorf("getting datacenter folders: %w", err)
	}

	path := fmt.Sprintf("/%s/vm/%s", s.Datacenter, folderName)
	if _, findErr := s.Finder.Folder(ctx, path); findErr == nil {
		log.V(1).Info("VM folder already exists", "path", path)
		return nil
	}

	_, err = folders.VmFolder.CreateFolder(ctx, folderName)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("creating VM folder %q: %w", folderName, err)
	}
	log.V(1).Info("created VM folder", "path", path)
	return nil
}

func DeleteVMFolder(ctx context.Context, s *Session, folderName string) error {
	log := klog.FromContext(ctx)

	path := fmt.Sprintf("/%s/vm/%s", s.Datacenter, folderName)
	folder, err := s.Finder.Folder(ctx, path)
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			log.V(1).Info("VM folder already absent", "path", path)
			return nil
		}
		return fmt.Errorf("finding VM folder %q: %w", path, err)
	}

	task, err := folder.Destroy(ctx)
	if err != nil {
		return fmt.Errorf("destroying VM folder %q: %w", path, err)
	}
	if err := task.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for VM folder %q destruction: %w", path, err)
	}
	log.V(1).Info("deleted VM folder", "path", path)
	return nil
}

// ClearSessions logs out and removes all cached sessions.
func ClearSessions(ctx context.Context) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	for key, s := range sessionCache {
		s.Close(ctx)
		delete(sessionCache, key)
	}
}
