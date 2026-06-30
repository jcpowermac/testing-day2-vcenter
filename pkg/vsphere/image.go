// image.go implements RHCOS OVA resolution, download, and import into vCenter.
//
// Adapted from the vcf-migration-operator's internal/vsphere/image.go, which
// was itself adapted from the MCO's pkg/controller/bootimage/vsphere_helpers.go.
package vsphere

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/stream-metadata-go/stream"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	maxTemplateNameLen = 80
	defaultCacheDir    = "/tmp/ova-scratch/image_cache"
)

// TemplateNameForFailureDomain returns the template VM name for a given failure
// domain, using the same naming convention as the MCO/installer.
func TemplateNameForFailureDomain(infraID, fdName string) string {
	return fmt.Sprintf("%s-rhcos-%s", infraID, fdName)
}

// ValidateTemplateName checks that the template name does not exceed the
// vSphere VM name length limit.
func ValidateTemplateName(name string) error {
	if len(name) > maxTemplateNameLen {
		return fmt.Errorf("template name %q exceeds %d character vSphere VM name limit (length: %d)", name, maxTemplateNameLen, len(name))
	}
	return nil
}

// ResolveRHCOSOVAFromConfigMap parses the coreos-bootimages ConfigMap and
// returns the vmware OVA artifact for the given architecture.
func ResolveRHCOSOVAFromConfigMap(cm *corev1.ConfigMap, arch string) (*stream.Artifact, error) {
	if cm == nil {
		return nil, fmt.Errorf("coreos-bootimages ConfigMap is nil")
	}

	streamJSON, ok := cm.Data["stream"]
	if !ok || streamJSON == "" {
		return nil, fmt.Errorf("coreos-bootimages ConfigMap missing 'stream' key")
	}

	streamData := new(stream.Stream)
	if err := json.Unmarshal([]byte(streamJSON), streamData); err != nil {
		return nil, fmt.Errorf("failed to parse CoreOS stream metadata from coreos-bootimages ConfigMap: %w", err)
	}

	ova, err := streamData.QueryDisk(arch, "vmware", "ova")
	if err != nil {
		return nil, fmt.Errorf("vmware OVA artifact not found for architecture %s: %w", arch, err)
	}

	if ova.Location == "" {
		return nil, fmt.Errorf("vmware OVA artifact for architecture %s has empty download URL", arch)
	}

	return ova, nil
}

// DownloadOVA downloads an OVA file to the default cache directory with
// optional SHA256 verification.
func DownloadOVA(ctx context.Context, ovaURL, sha256Expected string) (string, error) {
	return DownloadOVAToDir(ctx, ovaURL, sha256Expected, defaultCacheDir)
}

// DownloadOVAToDir downloads an OVA file to the specified cache directory.
func DownloadOVAToDir(ctx context.Context, ovaURL, sha256Expected, cacheDir string) (string, error) {
	log := klog.FromContext(ctx)

	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("creating cache directory %s: %w", cacheDir, err)
	}

	urlPath := ovaURL
	if idx := strings.IndexByte(urlPath, '?'); idx >= 0 {
		urlPath = urlPath[:idx]
	}
	filename := filepath.Base(urlPath)
	if filename == "" || filename == "." || filename == "/" {
		filename = "rhcos.ova"
	}
	localPath := filepath.Join(cacheDir, filename)

	lockPath := localPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating lock file %s: %w", lockPath, err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("acquiring flock on %s: %w", lockPath, err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	if sha256Expected != "" {
		if hash, err := hashFile(localPath); err == nil && hash == sha256Expected {
			log.V(1).Info("OVA already cached with correct hash", "path", localPath)
			return localPath, nil
		}
	} else if _, err := os.Stat(localPath); err == nil {
		log.V(1).Info("OVA already cached (no hash verification)", "path", localPath)
		return localPath, nil
	}

	log.Info("downloading OVA", "url", sanitizeOVAURL(ovaURL), "dest", localPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ovaURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading OVA from %s: %w", ovaURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading OVA from %s: HTTP %d %s", ovaURL, resp.StatusCode, resp.Status)
	}

	tmpFile, err := os.CreateTemp(cacheDir, "ova-download-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file for download: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	writer := io.Writer(tmpFile)
	if sha256Expected != "" {
		writer = io.MultiWriter(tmpFile, hasher)
	}

	written, err := io.Copy(writer, resp.Body)
	if err != nil {
		return "", fmt.Errorf("writing OVA to %s: %w", tmpPath, err)
	}
	log.V(1).Info("OVA download complete", "bytes", written)

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}

	if sha256Expected != "" {
		actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
		if actualHash != sha256Expected {
			return "", fmt.Errorf("OVA SHA256 mismatch: expected %s, got %s", sha256Expected, actualHash)
		}
		log.V(1).Info("OVA SHA256 verified", "hash", actualHash)
	}

	if err := os.Rename(tmpPath, localPath); err != nil {
		return "", fmt.Errorf("moving downloaded OVA to %s: %w", localPath, err)
	}

	return localPath, nil
}

// FindTemplateByName looks up a VM/template by name within the session's
// datacenter. Returns the inventory path and whether the VM was found.
func FindTemplateByName(ctx context.Context, s *Session, templateName string) (string, bool, error) {
	if s == nil || s.Finder == nil {
		return "", false, fmt.Errorf("session and Finder must not be nil")
	}
	log := klog.FromContext(ctx)

	vm, err := s.Finder.VirtualMachine(ctx, templateName)
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			log.V(2).Info("template not found", "name", templateName)
			return "", false, nil
		}
		return "", false, fmt.Errorf("finding VM %q: %w", templateName, err)
	}

	var vmProps mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.template"}, &vmProps); err != nil {
		return "", false, fmt.Errorf("getting VM properties for %q: %w", templateName, err)
	}

	if vmProps.Config == nil || !vmProps.Config.Template {
		return "", true, fmt.Errorf(
			"VM %q exists but is not a template; rename or remove the existing VM, "+
				"or set topology.template manually to use it", templateName,
		)
	}

	log.V(1).Info("found existing template", "name", templateName, "path", vm.InventoryPath)
	return vm.InventoryPath, true, nil
}

// ImportOVAParams holds parameters for importing an OVA into vCenter.
type ImportOVAParams struct {
	Session          *Session
	OVAPath          string
	TemplateName     string
	ComputeCluster   string
	Datastore        string
	Network          string
	Folder           string
	ResourcePool     string
	DiskProvisioning string
}

// ImportOVA imports an OVA file into vCenter as a VM, then marks it as a template.
func ImportOVA(ctx context.Context, p ImportOVAParams) (*object.VirtualMachine, error) {
	log := klog.FromContext(ctx)
	s := p.Session

	if s == nil || s.Client == nil || s.Finder == nil {
		return nil, fmt.Errorf("session, Client, and Finder must not be nil")
	}

	ovfDescriptor, err := readOVFFromOVA(p.OVAPath)
	if err != nil {
		return nil, fmt.Errorf("reading OVF from OVA %s: %w", p.OVAPath, err)
	}

	cluster, err := s.Finder.ClusterComputeResource(ctx, p.ComputeCluster)
	if err != nil {
		return nil, fmt.Errorf("finding cluster %q: %w", p.ComputeCluster, err)
	}

	datastore, err := s.Finder.Datastore(ctx, p.Datastore)
	if err != nil {
		return nil, fmt.Errorf("finding datastore %q: %w", p.Datastore, err)
	}

	network, err := s.Finder.Network(ctx, p.Network)
	if err != nil {
		return nil, fmt.Errorf("finding network %q: %w", p.Network, err)
	}

	var folder *object.Folder
	if p.Folder != "" {
		folder, err = s.Finder.Folder(ctx, p.Folder)
		if err != nil {
			return nil, fmt.Errorf("finding folder %q: %w", p.Folder, err)
		}
	} else {
		dc, dcErr := s.Finder.Datacenter(ctx, s.Datacenter)
		if dcErr != nil {
			return nil, fmt.Errorf("finding datacenter %q: %w", s.Datacenter, dcErr)
		}
		folders, fErr := dc.Folders(ctx)
		if fErr != nil {
			return nil, fmt.Errorf("getting datacenter folders: %w", fErr)
		}
		folder = folders.VmFolder
	}

	var resourcePool *object.ResourcePool
	if p.ResourcePool != "" {
		resourcePool, err = s.Finder.ResourcePool(ctx, p.ResourcePool)
		if err != nil {
			return nil, fmt.Errorf("finding resource pool %q: %w", p.ResourcePool, err)
		}
	} else {
		resourcePool, err = cluster.ResourcePool(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting cluster default resource pool: %w", err)
		}
	}

	host, err := findAvailableHost(ctx, s, cluster, network, datastore)
	if err != nil {
		return nil, fmt.Errorf("finding available host: %w", err)
	}

	cisp, err := createImportSpec(ctx, s, ovfDescriptor, resourcePool, datastore, network, p.TemplateName, p.DiskProvisioning)
	if err != nil {
		return nil, fmt.Errorf("creating OVF import spec: %w", err)
	}

	log.V(1).Info("starting OVA import",
		"template", p.TemplateName,
		"cluster", p.ComputeCluster,
		"datastore", p.Datastore,
		"host", host.InventoryPath,
	)

	lease, err := resourcePool.ImportVApp(ctx, cisp.ImportSpec, folder, host)
	if err != nil {
		return nil, fmt.Errorf("importing VApp: %w", err)
	}

	info, err := lease.Wait(ctx, cisp.FileItem)
	if err != nil {
		return nil, fmt.Errorf("waiting for NFC lease: %w", err)
	}

	leaseCompleted := false
	defer func() {
		if !leaseCompleted {
			if abortErr := lease.Abort(ctx, nil); abortErr != nil {
				log.V(1).Info("failed to abort NFC lease", "error", abortErr)
			}
		}
	}()

	updater := lease.StartUpdater(ctx, info)
	defer updater.Done()

	if err := upload(ctx, lease, info, p.OVAPath); err != nil {
		return nil, fmt.Errorf("uploading OVA files: %w", err)
	}

	if err := lease.Complete(ctx); err != nil {
		return nil, fmt.Errorf("completing NFC lease: %w", err)
	}
	leaseCompleted = true

	vm, err := s.Finder.VirtualMachine(ctx, p.TemplateName)
	if err != nil {
		return nil, fmt.Errorf("finding imported VM %q: %w", p.TemplateName, err)
	}

	if err := disableSecureBootIfNeeded(ctx, vm, ovfDescriptor); err != nil {
		log.V(1).Info("warning: failed to check/disable secure boot", "error", err)
	}

	if err := vm.MarkAsTemplate(ctx); err != nil {
		return nil, fmt.Errorf("marking VM %q as template: %w", p.TemplateName, err)
	}
	log.V(1).Info("marked VM as template", "name", p.TemplateName, "path", vm.InventoryPath)

	return vm, nil
}

func readOVFFromOVA(ovaPath string) (string, error) {
	f, err := os.Open(ovaPath)
	if err != nil {
		return "", fmt.Errorf("opening OVA %s: %w", ovaPath, err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading OVA tar: %w", err)
		}

		if strings.HasSuffix(header.Name, ".ovf") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return "", fmt.Errorf("reading OVF from OVA: %w", err)
			}
			return string(data), nil
		}
	}

	return "", fmt.Errorf("no .ovf file found in OVA %s", ovaPath)
}

func findAvailableHost(ctx context.Context, s *Session, cluster *object.ClusterComputeResource, network object.NetworkReference, datastore *object.Datastore) (*object.HostSystem, error) {
	log := klog.FromContext(ctx)

	var clusterProps mo.ClusterComputeResource
	if err := cluster.Properties(ctx, cluster.Reference(), []string{"host"}, &clusterProps); err != nil {
		return nil, fmt.Errorf("getting cluster hosts: %w", err)
	}

	if len(clusterProps.Host) == 0 {
		return nil, fmt.Errorf("cluster %q has no hosts", cluster.InventoryPath)
	}

	dsRef := datastore.Reference()
	netRef := network.Reference()

	for _, hostRef := range clusterProps.Host {
		host := object.NewHostSystem(s.Client.Client, hostRef)
		var hostProps mo.HostSystem
		if err := host.Properties(ctx, hostRef, []string{"name", "runtime.connectionState", "runtime.inMaintenanceMode", "datastore", "network"}, &hostProps); err != nil {
			log.V(2).Info("skipping host, failed to get properties", "host", hostRef.Value, "error", err)
			continue
		}

		if hostProps.Runtime.ConnectionState != "connected" {
			continue
		}
		if hostProps.Runtime.InMaintenanceMode {
			continue
		}

		dsAvailable := false
		for _, ds := range hostProps.Datastore {
			if ds == dsRef {
				dsAvailable = true
				break
			}
		}
		if !dsAvailable {
			continue
		}

		netAvailable := false
		for _, net := range hostProps.Network {
			if net == netRef {
				netAvailable = true
				break
			}
		}
		if !netAvailable {
			continue
		}

		log.V(1).Info("selected host for OVA import", "host", hostProps.Name)
		return host, nil
	}

	return nil, fmt.Errorf("no available host in cluster %q with access to datastore and network", cluster.InventoryPath)
}

func createImportSpec(
	ctx context.Context,
	s *Session,
	ovfDescriptor string,
	resourcePool *object.ResourcePool,
	datastore *object.Datastore,
	network object.NetworkReference,
	vmName string,
	diskProvisioning string,
) (*types.OvfCreateImportSpecResult, error) {
	ovfManager := ovf.NewManager(s.Client.Client)

	spec := types.OvfCreateImportSpecParams{
		DiskProvisioning: diskProvisioning,
		EntityName:       vmName,
	}

	var envelope ovf.Envelope
	if err := xml.Unmarshal([]byte(ovfDescriptor), &envelope); err == nil && envelope.Network != nil {
		for _, net := range envelope.Network.Networks {
			spec.NetworkMapping = append(spec.NetworkMapping, types.OvfNetworkMapping{
				Name:    net.Name,
				Network: network.Reference(),
			})
		}
	}

	cisp, err := ovfManager.CreateImportSpec(ctx, ovfDescriptor, resourcePool, datastore, &spec)
	if err != nil {
		return nil, fmt.Errorf("creating import spec: %w", err)
	}

	if cisp.Error != nil && len(cisp.Error) > 0 {
		return nil, fmt.Errorf("import spec errors: %v", cisp.Error)
	}

	return cisp, nil
}

func upload(ctx context.Context, lease *nfc.Lease, info *nfc.LeaseInfo, ovaPath string) error {
	f, err := os.Open(ovaPath)
	if err != nil {
		return fmt.Errorf("opening OVA for upload: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)

	itemMap := make(map[string]nfc.FileItem)
	for _, item := range info.Items {
		itemMap[item.Path] = item
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading OVA tar: %w", err)
		}

		item, ok := itemMap[header.Name]
		if !ok {
			continue
		}

		if err := lease.Upload(ctx, item, tr, soap.Upload{ContentLength: header.Size}); err != nil {
			return fmt.Errorf("uploading %s: %w", header.Name, err)
		}
	}

	return nil
}

func disableSecureBootIfNeeded(ctx context.Context, vm *object.VirtualMachine, ovfDescriptor string) error {
	if !strings.Contains(ovfDescriptor, "efi.secureBoot") &&
		!strings.Contains(ovfDescriptor, "secureBoot") {
		return nil
	}

	log := klog.FromContext(ctx)
	log.V(1).Info("disabling secure boot on imported VM (RHCOS OVA)")

	var vmProps mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.bootOptions"}, &vmProps); err != nil {
		return fmt.Errorf("getting VM boot options: %w", err)
	}

	spec := types.VirtualMachineConfigSpec{
		BootOptions: &types.VirtualMachineBootOptions{
			EfiSecureBootEnabled: types.NewBool(false),
		},
	}

	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfiguring VM to disable secure boot: %w", err)
	}

	return task.Wait(ctx)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func sanitizeOVAURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<unparseable-url>"
	}
	sanitized := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		sanitized += "?<redacted>"
	}
	return sanitized
}
