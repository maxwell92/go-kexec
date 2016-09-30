package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/cli/command"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/notary/client"
	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/trustpinning"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
	"github.com/docker/notary/tuf/store"
)

var (
	IndexName = "docker.io"
)

var emptyServiceConfig = newServiceConfig(ServiceOptions{})

// for mocking in unit tests
var lookupIP = net.LookupIP

type RepositoryInfo struct {
	reference.Named
	Index    *registrytypes.IndexInfo
	Official bool
}

type IndexInfo struct {
	// Name is the name of the registry, such as "docker.io"
	Name string
	// Mirrors is a list of mirrors, expressed as URIs
	Mirrors []string
	// Secure is set to false if the registry is part of the list of
	// insecure registries. Insecure registries accept HTTP and/or accept
	// HTTPS with certificates from unknown CAs.
	Secure bool
	// Official indicates whether this is an official registry
	Official bool
}

// ServiceOptions holds command line options.
type ServiceOptions struct {
	Mirrors            []string `json:"registry-mirrors,omitempty"`
	InsecureRegistries []string `json:"insecure-registries,omitempty"`

	// V2Only controls access to legacy registries.  If it is set to true via the
	// command line flag the daemon will not attempt to contact v1 legacy registries
	V2Only bool `json:"disable-legacy-registry,omitempty"`
}

// serviceConfig holds daemon configuration for the registry service.
type serviceConfig struct {
	registrytypes.ServiceConfig
	V2Only bool
}

type simpleCredentialStore struct {
	auth types.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

func (scs simpleCredentialStore) RefreshToken(u *url.URL, service string) string {
	return scs.auth.IdentityToken
}

func (scs simpleCredentialStore) SetRefreshToken(*url.URL, string, string) {
}

// newServiceConfig returns a new instance of ServiceConfig
func newServiceConfig(options ServiceOptions) *serviceConfig {
	// Localhost is by default considered as an insecure registry
	// This is a stop-gap for people who are running a private registry on localhost (especially on Boot2docker).
	//
	// TODO: should we deprecate this once it is easier for people to set up a TLS registry or change
	// daemon flags on boot2docker?
	options.InsecureRegistries = append(options.InsecureRegistries, "127.0.0.0/8")

	config := &serviceConfig{
		ServiceConfig: registrytypes.ServiceConfig{
			InsecureRegistryCIDRs: make([]*registrytypes.NetIPNet, 0),
			IndexConfigs:          make(map[string]*registrytypes.IndexInfo, 0),
			// Hack: Bypass setting the mirrors to IndexConfigs since they are going away
			// and Mirrors are only for the official registry anyways.
			Mirrors: options.Mirrors,
		},
		V2Only: options.V2Only,
	}
	// Split --insecure-registry into CIDR and registry-specific settings.
	for _, r := range options.InsecureRegistries {
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR.
			config.InsecureRegistryCIDRs = append(config.InsecureRegistryCIDRs, (*registrytypes.NetIPNet)(ipnet))
		} else {
			// Assume `host:port` if not CIDR.
			config.IndexConfigs[r] = &registrytypes.IndexInfo{
				Name:     r,
				Mirrors:  make([]string, 0),
				Secure:   false,
				Official: false,
			}
		}
	}

	// Configure public registry.
	config.IndexConfigs[IndexName] = &registrytypes.IndexInfo{
		Name:     IndexName,
		Mirrors:  config.Mirrors,
		Secure:   true,
		Official: true,
	}

	return config
}

func printInfo(remote string) {

	fmt.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	fmt.Println(remote)
	fmt.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")

	/*
		ref data type:
		type Named interface {
			Name() string
			String() string
			FullName() string
			Hostname() string
			RemoteName() string
		}
	*/
	ref, err := reference.ParseNamed(remote)

	if err != nil {
		panic(err)
	}
	fmt.Printf("ref.Name()       --> %s\n", ref.Name())
	fmt.Printf("ref.String()     --> %s\n", ref.String())
	fmt.Printf("ref.FullName()   --> %s\n", ref.FullName())
	fmt.Printf("ref.Hostname()   --> %s\n", ref.Hostname())
	fmt.Printf("ref.RemoteName() --> %s\n", ref.RemoteName())

	/*
		repoInfo data type:
		type RepositoryInfo struct {
			reference.Named
			Index *registrytypes.IndexInfo
			Official bool
		}

		type IndexInfo struct {
			// Name is the name of the registry, such as "docker.io"
			Name string
			// Mirrors is a list of mirrors, expressed as URIs
			Mirrors []string
			// Secure is set to false if the registry is part of the list of
			// insecure registries. Insecure registries accept HTTP and/or accept
			// HTTPS with certificates from unknown CAs.
			Secure bool
			// Official indicates whether this is an official registry
			Official bool
		}*/
	repoInfo, err := ParseRepositoryInfo(ref)
	if err != nil {
		panic(err)
	}
	fmt.Printf("repoInfo.Index.Name     --> %s\n", repoInfo.Index.Name)
	fmt.Printf("repoInfo.Index.Mirrors  --> %v\n", repoInfo.Index.Mirrors)
	fmt.Printf("repoInfo.Index.Secure   --> %s\n", repoInfo.Index.Secure)
	fmt.Printf("repoInfo.Index.Official --> %s\n", repoInfo.Index.Official)
}

func ParseRepositoryInfo(reposName reference.Named) (*RepositoryInfo, error) {
	return newRepositoryInfo(emptyServiceConfig, reposName)
}

// ValidateIndexName validates an index name.
func ValidateIndexName(val string) (string, error) {
	if val == reference.LegacyDefaultHostname {
		val = reference.DefaultHostname
	}
	if strings.HasPrefix(val, "-") || strings.HasSuffix(val, "-") {
		return "", fmt.Errorf("Invalid index name (%s). Cannot begin or end with a hyphen.", val)
	}
	return val, nil
}

// newIndexInfo returns IndexInfo configuration from indexName
func newIndexInfo(config *serviceConfig, indexName string) (*registrytypes.IndexInfo, error) {
	var err error
	indexName, err = ValidateIndexName(indexName)
	if err != nil {
		return nil, err
	}

	// Return any configured index info, first.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index, nil
	}

	// Construct a non-configured index info.
	index := &registrytypes.IndexInfo{
		Name:     indexName,
		Mirrors:  make([]string, 0),
		Official: false,
	}
	index.Secure = isSecureIndex(config, indexName)
	return index, nil
}

func newRepositoryInfo(config *serviceConfig, name reference.Named) (*RepositoryInfo, error) {
	index, err := newIndexInfo(config, name.Hostname())
	if err != nil {
		return nil, err
	}
	official := !strings.ContainsRune(name.Name(), '/')
	return &RepositoryInfo{name, index, official}, nil
}

func isSecureIndex(config *serviceConfig, indexName string) bool {
	// Check for configured index, first.  This is needed in case isSecureIndex
	// is called from anything besides newIndexInfo, in order to honor per-index configurations.
	if index, ok := config.IndexConfigs[indexName]; ok {
		return index.Secure
	}

	host, _, err := net.SplitHostPort(indexName)
	if err != nil {
		// assume indexName is of the form `host` without the port and go on.
		host = indexName
	}

	addrs, err := lookupIP(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip != nil {
			addrs = []net.IP{ip}
		}

		// if ip == nil, then `host` is neither an IP nor it could be looked up,
		// either because the index is unreachable, or because the index is behind an HTTP proxy.
		// So, len(addrs) == 0 and we're not aborting.
	}

	// Try CIDR notation only if addrs has any elements, i.e. if `host`'s IP could be determined.
	for _, addr := range addrs {
		for _, ipnet := range config.InsecureRegistryCIDRs {
			// check if the addr falls in the subnet
			if (*net.IPNet)(ipnet).Contains(addr) {
				return false
			}
		}
	}

	return true
}

func main() {
	printInfo("centos:centos7")
	printInfo("registry.paas.symcpe.com:443/xuant/centos:centos7")

	ref, err := reference.ParseNamed("registry.paas.symcpe.com:443/xuant/centos")
	if err != nil {
		panic(err)
	}

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		panic(err)
	}

	fmt.Println("=================")

	stdin, stdout, stderr := term.StdStreams()
	dockerCli := command.NewDockerCli(stdin, stdout, stderr)

	// TODO: initialize dockerCli
	opts := cliflags.NewClientOptions()
	opts.Common = &cliflags.CommonOptions{
		Hosts: []string{"unix:///var/run/docker.sock"},
	}
	dockerPreRun(opts)
	dockerCli.Initialize(opts)

	ctx := context.Background()

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(dockerCli, repoInfo.Index, "push")

	if command.IsTrusted() {
		panic(trustedPush(ctx, dockerCli, repoInfo, ref, authConfig, requestPrivilege))
	}

	responseBody, err := imagePushPrivileged(ctx, dockerCli, authConfig, ref.String(), requestPrivilege)
	if err != nil {
		panic(err)
	}

	defer responseBody.Close()
	panic(jsonmessage.DisplayJSONMessagesToStream(responseBody, dockerCli.Out(), nil))
}

// trustedPush handles content trust pushing of an image
func trustedPush(ctx context.Context, cli *command.DockerCli, repoInfo *registry.RepositoryInfo, ref reference.Named, authConfig types.AuthConfig, requestPrivilege types.RequestPrivilegeFunc) error {
	responseBody, err := imagePushPrivileged(ctx, cli, authConfig, ref.String(), requestPrivilege)
	if err != nil {
		return err
	}

	defer responseBody.Close()

	// If it is a trusted push we would like to find the target entry which match the
	// tag provided in the function and then do an AddTarget later.
	target := &client.Target{}
	// Count the times of calling for handleTarget,
	// if it is called more that once, that should be considered an error in a trusted push.
	cnt := 0
	handleTarget := func(aux *json.RawMessage) {
		cnt++
		if cnt > 1 {
			// handleTarget should only be called one. This will be treated as an error.
			return
		}

		var pushResult distribution.PushResult
		err := json.Unmarshal(*aux, &pushResult)
		if err == nil && pushResult.Tag != "" && pushResult.Digest.Validate() == nil {
			h, err := hex.DecodeString(pushResult.Digest.Hex())
			if err != nil {
				target = nil
				return
			}
			target.Name = registry.ParseReference(pushResult.Tag).String()
			target.Hashes = data.Hashes{string(pushResult.Digest.Algorithm()): h}
			target.Length = int64(pushResult.Size)
		}
	}

	var tag string
	switch x := ref.(type) {
	case reference.Canonical:
		return errors.New("cannot push a digest reference")
	case reference.NamedTagged:
		tag = x.Tag()
	}

	// We want trust signatures to always take an explicit tag,
	// otherwise it will act as an untrusted push.
	if tag == "" {
		if err = jsonmessage.DisplayJSONMessagesToStream(responseBody, cli.Out(), nil); err != nil {
			return err
		}
		fmt.Fprintln(cli.Out(), "No tag specified, skipping trust metadata push")
		return nil
	}

	if err = jsonmessage.DisplayJSONMessagesToStream(responseBody, cli.Out(), handleTarget); err != nil {
		return err
	}

	if cnt > 1 {
		return fmt.Errorf("internal error: only one call to handleTarget expected")
	}

	if target == nil {
		fmt.Fprintln(cli.Out(), "No targets found, please provide a specific tag in order to sign it")
		return nil
	}

	fmt.Fprintln(cli.Out(), "Signing and pushing trust metadata")

	repo, err := GetNotaryRepository(cli, repoInfo, authConfig, "push", "pull")
	if err != nil {
		fmt.Fprintf(cli.Out(), "Error establishing connection to notary repository: %s\n", err)
		return err
	}

	// get the latest repository metadata so we can figure out which roles to sign
	err = repo.Update(false)

	switch err.(type) {
	case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
		keys := repo.CryptoService.ListKeys(data.CanonicalRootRole)
		var rootKeyID string
		// always select the first root key
		if len(keys) > 0 {
			sort.Strings(keys)
			rootKeyID = keys[0]
		} else {
			rootPublicKey, err := repo.CryptoService.Create(data.CanonicalRootRole, "", data.ECDSAKey)
			if err != nil {
				return err
			}
			rootKeyID = rootPublicKey.ID()
		}

		// Initialize the notary repository with a remotely managed snapshot key
		if err := repo.Initialize(rootKeyID, data.CanonicalSnapshotRole); err != nil {
			return notaryError(repoInfo.FullName(), err)
		}
		fmt.Fprintf(cli.Out(), "Finished initializing %q\n", repoInfo.FullName())
		err = repo.AddTarget(target, data.CanonicalTargetsRole)
	case nil:
		// already initialized and we have successfully downloaded the latest metadata
		err = addTargetToAllSignableRoles(repo, target)
	default:
		return notaryError(repoInfo.FullName(), err)
	}

	if err == nil {
		err = repo.Publish()
	}

	if err != nil {
		fmt.Fprintf(cli.Out(), "Failed to sign %q:%s - %s\n", repoInfo.FullName(), tag, err.Error())
		return notaryError(repoInfo.FullName(), err)
	}

	fmt.Fprintf(cli.Out(), "Successfully signed %q:%s\n", repoInfo.FullName(), tag)
	return nil
}

// GetNotaryRepository returns a NotaryRepository which stores all the
// information needed to operate on a notary repository.
// It creates an HTTP transport providing authentication support.
// TODO: move this too
func GetNotaryRepository(streams command.Streams, repoInfo *registry.RepositoryInfo, authConfig types.AuthConfig, actions ...string) (*client.NotaryRepository, error) {
	server, err := trustServer(repoInfo.Index)
	if err != nil {
		return nil, err
	}

	var cfg = tlsconfig.ClientDefault()
	cfg.InsecureSkipVerify = !repoInfo.Index.Secure

	// Get certificate base directory
	certDir, err := certificateDirectory(server)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("reading certificate directory: %s", certDir)

	if err := registry.ReadCertsDirectory(cfg, certDir); err != nil {
		return nil, err
	}

	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
		DisableKeepAlives:   true,
	}

	// Skip configuration headers since request is not going to Docker daemon
	modifiers := registry.DockerHeaders(command.UserAgent(), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}
	endpointStr := server + "/v2/"
	req, err := http.NewRequest("GET", endpointStr, nil)
	if err != nil {
		return nil, err
	}

	challengeManager := auth.NewSimpleChallengeManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		// Ignore error on ping to operate in offline mode
		logrus.Debugf("Error pinging notary server %q: %s", endpointStr, err)
	} else {
		defer resp.Body.Close()

		// Add response to the challenge manager to parse out
		// authentication header and register authentication method
		if err := challengeManager.AddResponse(resp); err != nil {
			return nil, err
		}
	}

	creds := simpleCredentialStore{auth: authConfig}
	tokenHandler := auth.NewTokenHandler(authTransport, creds, repoInfo.FullName(), actions...)
	basicHandler := auth.NewBasicHandler(creds)
	modifiers = append(modifiers, transport.RequestModifier(auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)))
	tr := transport.NewTransport(base, modifiers...)

	return client.NewNotaryRepository(
		trustDirectory(),
		repoInfo.FullName(),
		server,
		tr,
		getPassphraseRetriever(streams),
		trustpinning.TrustPinConfig{})
}

// imagePushPrivileged push the image
func imagePushPrivileged(ctx context.Context, cli *command.DockerCli, authConfig types.AuthConfig, ref string, requestPrivilege types.RequestPrivilegeFunc) (io.ReadCloser, error) {
	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, err
	}
	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	return cli.Client().ImagePush(ctx, ref, options)
}

// notaryError formats an error message received from the notary service
func notaryError(repoName string, err error) error {
	switch err.(type) {
	case *json.SyntaxError:
		logrus.Debugf("Notary syntax error: %s", err)
		return fmt.Errorf("Error: no trust data available for remote repository %s. Try running notary server and setting DOCKER_CONTENT_TRUST_SERVER to its HTTPS address?", repoName)
	case signed.ErrExpired:
		return fmt.Errorf("Error: remote repository %s out-of-date: %v", repoName, err)
	case trustmanager.ErrKeyNotFound:
		return fmt.Errorf("Error: signing keys for remote repository %s not found: %v", repoName, err)
	case *net.OpError:
		return fmt.Errorf("Error: error contacting notary server: %v", err)
	case store.ErrMetaNotFound:
		return fmt.Errorf("Error: trust data missing for remote repository %s or remote repository not found: %v", repoName, err)
	case signed.ErrInvalidKeyType:
		return fmt.Errorf("Warning: potential malicious behavior - trust data mismatch for remote repository %s: %v", repoName, err)
	case signed.ErrNoKeys:
		return fmt.Errorf("Error: could not find signing keys for remote repository %s, or could not decrypt signing key: %v", repoName, err)
	case signed.ErrLowVersion:
		return fmt.Errorf("Warning: potential malicious behavior - trust data version is lower than expected for remote repository %s: %v", repoName, err)
	case signed.ErrRoleThreshold:
		return fmt.Errorf("Warning: potential malicious behavior - trust data has insufficient signatures for remote repository %s: %v", repoName, err)
	case client.ErrRepositoryNotExist:
		return fmt.Errorf("Error: remote trust data does not exist for %s: %v", repoName, err)
	case signed.ErrInsufficientSignatures:
		return fmt.Errorf("Error: could not produce valid signature for %s.  If Yubikey was used, was touch input provided?: %v", repoName, err)
	}

	return err
}

// Attempt to add the image target to all the top level delegation roles we can
// (based on whether we have the signing key and whether the role's path allows
// us to).
// If there are no delegation roles, we add to the targets role.
func addTargetToAllSignableRoles(repo *client.NotaryRepository, target *client.Target) error {
	var signableRoles []string

	// translate the full key names, which includes the GUN, into just the key IDs
	allCanonicalKeyIDs := make(map[string]struct{})
	for fullKeyID := range repo.CryptoService.ListAllKeys() {
		allCanonicalKeyIDs[path.Base(fullKeyID)] = struct{}{}
	}

	allDelegationRoles, err := repo.GetDelegationRoles()
	if err != nil {
		return err
	}

	// if there are no delegation roles, then just try to sign it into the targets role
	if len(allDelegationRoles) == 0 {
		return repo.AddTarget(target, data.CanonicalTargetsRole)
	}

	// there are delegation roles, find every delegation role we have a key for, and
	// attempt to sign into into all those roles.
	for _, delegationRole := range allDelegationRoles {
		// We do not support signing any delegation role that isn't a direct child of the targets role.
		// Also don't bother checking the keys if we can't add the target
		// to this role due to path restrictions
		if path.Dir(delegationRole.Name) != data.CanonicalTargetsRole || !delegationRole.CheckPaths(target.Name) {
			continue
		}

		for _, canonicalKeyID := range delegationRole.KeyIDs {
			if _, ok := allCanonicalKeyIDs[canonicalKeyID]; ok {
				signableRoles = append(signableRoles, delegationRole.Name)
				break
			}
		}
	}

	if len(signableRoles) == 0 {
		return fmt.Errorf("no valid signing keys for delegation roles")
	}

	return repo.AddTarget(target, signableRoles...)
}

// certificateDirectory returns the directory containing
// TLS certificates for the given server. An error is
// returned if there was an error parsing the server string.
func certificateDirectory(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}

	return filepath.Join(cliconfig.ConfigDir(), "tls", u.Host), nil
}

func trustServer(index *registrytypes.IndexInfo) (string, error) {
	if s := os.Getenv("DOCKER_CONTENT_TRUST_SERVER"); s != "" {
		urlObj, err := url.Parse(s)
		if err != nil || urlObj.Scheme != "https" {
			return "", fmt.Errorf("valid https URL required for trust server, got %s", s)
		}

		return s, nil
	}
	if index.Official {
		return registry.NotaryServer, nil
	}
	return "https://" + index.Name, nil
}

func trustDirectory() string {
	return filepath.Join(cliconfig.ConfigDir(), "trust")
}

func getPassphraseRetriever(streams command.Streams) passphrase.Retriever {
	aliasMap := map[string]string{
		"root":     "root",
		"snapshot": "repository",
		"targets":  "repository",
		"default":  "repository",
	}
	baseRetriever := passphrase.PromptRetrieverWithInOut(streams.In(), streams.Out(), aliasMap)
	env := map[string]string{
		"root":     os.Getenv("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE"),
		"snapshot": os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"targets":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"default":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
	}

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if v := env[alias]; v != "" {
			return v, numAttempts > 1, nil
		}
		// For non-root roles, we can also try the "default" alias if it is specified
		if v := env["default"]; v != "" && alias != data.CanonicalRootRole {
			return v, numAttempts > 1, nil
		}
		return baseRetriever(keyName, alias, createNew, numAttempts)
	}
}

func dockerPreRun(opts *cliflags.ClientOptions) {
	cliflags.SetDaemonLogLevel(opts.Common.LogLevel)

	if opts.ConfigDir != "" {
		cliconfig.SetConfigDir(opts.ConfigDir)
	}

	if opts.Common.Debug {
		utils.EnableDebug()
	}
}
