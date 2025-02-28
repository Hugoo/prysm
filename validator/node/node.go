// Package node is the main process which handles the lifecycle of
// the runtime services in a validator client process, gracefully shutting
// everything down upon close.
package node

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/ethereum/go-ethereum/common/hexutil"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/api/gateway"
	"github.com/prysmaticlabs/prysm/api/gateway/apimiddleware"
	"github.com/prysmaticlabs/prysm/async/event"
	"github.com/prysmaticlabs/prysm/cmd"
	"github.com/prysmaticlabs/prysm/cmd/validator/flags"
	"github.com/prysmaticlabs/prysm/config/features"
	"github.com/prysmaticlabs/prysm/config/params"
	"github.com/prysmaticlabs/prysm/encoding/bytesutil"
	"github.com/prysmaticlabs/prysm/io/file"
	"github.com/prysmaticlabs/prysm/monitoring/backup"
	"github.com/prysmaticlabs/prysm/monitoring/prometheus"
	tracing2 "github.com/prysmaticlabs/prysm/monitoring/tracing"
	ethpbservice "github.com/prysmaticlabs/prysm/proto/eth/service"
	pb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
	validatorpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1/validator-client"
	"github.com/prysmaticlabs/prysm/runtime"
	"github.com/prysmaticlabs/prysm/runtime/debug"
	"github.com/prysmaticlabs/prysm/runtime/prereqs"
	"github.com/prysmaticlabs/prysm/runtime/version"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/client"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	g "github.com/prysmaticlabs/prysm/validator/graffiti"
	"github.com/prysmaticlabs/prysm/validator/keymanager/imported"
	remote_web3signer "github.com/prysmaticlabs/prysm/validator/keymanager/remote-web3signer"
	"github.com/prysmaticlabs/prysm/validator/rpc"
	validatorMiddleware "github.com/prysmaticlabs/prysm/validator/rpc/apimiddleware"
	"github.com/prysmaticlabs/prysm/validator/web"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

// ValidatorClient defines an instance of an Ethereum validator that manages
// the entire lifecycle of services attached to it participating in proof of stake.
type ValidatorClient struct {
	cliCtx            *cli.Context
	ctx               context.Context
	cancel            context.CancelFunc
	db                *kv.Store
	services          *runtime.ServiceRegistry // Lifecycle and service store.
	lock              sync.RWMutex
	wallet            *wallet.Wallet
	walletInitialized *event.Feed
	stop              chan struct{} // Channel to wait for termination notifications.
}

// NewValidatorClient creates a new instance of the Prysm validator client.
func NewValidatorClient(cliCtx *cli.Context) (*ValidatorClient, error) {
	// TODO(#9883) - Maybe we can pass in a new validator client config instead of the cliCTX to abstract away the use of flags here .
	if err := tracing2.Setup(
		"validator", // service name
		cliCtx.String(cmd.TracingProcessNameFlag.Name),
		cliCtx.String(cmd.TracingEndpointFlag.Name),
		cliCtx.Float64(cmd.TraceSampleFractionFlag.Name),
		cliCtx.Bool(cmd.EnableTracingFlag.Name),
	); err != nil {
		return nil, err
	}

	verbosity := cliCtx.String(cmd.VerbosityFlag.Name)
	level, err := logrus.ParseLevel(verbosity)
	if err != nil {
		return nil, err
	}
	logrus.SetLevel(level)

	// Warn if user's platform is not supported
	prereqs.WarnIfPlatformNotSupported(cliCtx.Context)

	registry := runtime.NewServiceRegistry()
	ctx, cancel := context.WithCancel(cliCtx.Context)
	validatorClient := &ValidatorClient{
		cliCtx:            cliCtx,
		ctx:               ctx,
		cancel:            cancel,
		services:          registry,
		walletInitialized: new(event.Feed),
		stop:              make(chan struct{}),
	}

	features.ConfigureValidator(cliCtx)
	cmd.ConfigureValidator(cliCtx)

	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}

	// If the --web flag is enabled to administer the validator
	// client via a web portal, we start the validator client in a different way.
	if cliCtx.IsSet(flags.EnableWebFlag.Name) {
		if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) || cliCtx.IsSet(flags.Web3SignerPublicValidatorKeysFlag.Name) {
			return nil, errors.New("web3signer cannot be used with --web")
		}
		log.Info("Enabling web portal to manage the validator client")
		if err := validatorClient.initializeForWeb(cliCtx); err != nil {
			return nil, err
		}
		return validatorClient, nil
	}

	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}

	// Initializes any forks here.
	params.BeaconConfig().InitializeForkSchedule()

	if err := validatorClient.initializeFromCLI(cliCtx); err != nil {
		return nil, err
	}

	return validatorClient, nil
}

// Start every service in the validator client.
func (c *ValidatorClient) Start() {
	c.lock.Lock()

	log.WithFields(logrus.Fields{
		"version": version.Version(),
	}).Info("Starting validator node")

	c.services.StartAll()

	stop := c.stop
	c.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")
		debug.Exit(c.cliCtx) // Ensure trace and CPU profile data are flushed.
		go c.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic.")
			}
		}
		panic("Panic closing the validator client")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (c *ValidatorClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.services.StopAll()
	log.Info("Stopping Prysm validator")
	c.cancel()
	close(c.stop)
}

func (c *ValidatorClient) initializeFromCLI(cliCtx *cli.Context) error {
	var err error
	dataDir := cliCtx.String(flags.WalletDirFlag.Name)
	if !cliCtx.IsSet(flags.InteropNumValidators.Name) {
		// Custom Check For Web3Signer
		if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) || cliCtx.IsSet(flags.Web3SignerPublicValidatorKeysFlag.Name) {
			if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) && cliCtx.IsSet(flags.Web3SignerPublicValidatorKeysFlag.Name) {
				c.wallet = wallet.NewWalletForWeb3Signer()
			} else {
				return errors.New("--validators-external-signer-url and --validators-external-signer-public-keys must be used together")
			}
		} else {
			w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
				return nil, wallet.ErrNoWalletFound
			})
			if err != nil {
				return errors.Wrap(err, "could not open wallet")
			}
			c.wallet = w
			// TODO(#9883) - Remove this when we have a better way to handle this.
			log.WithFields(logrus.Fields{
				"wallet":          w.AccountsDir(),
				"keymanager-kind": w.KeymanagerKind().String(),
			}).Info("Opened validator wallet")
			dataDir = c.wallet.AccountsDir()
		}
	}
	if cliCtx.String(cmd.DataDirFlag.Name) != cmd.DefaultDataDir() {
		dataDir = cliCtx.String(cmd.DataDirFlag.Name)
	}
	clearFlag := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearFlag := cliCtx.Bool(cmd.ForceClearDB.Name)
	if clearFlag || forceClearFlag {
		if dataDir == "" && c.wallet != nil {
			dataDir = c.wallet.AccountsDir()
			if dataDir == "" {
				log.Fatal(
					"Could not determine your system'c HOME path, please specify a --datadir you wish " +
						"to use for your validator data",
				)
			}

		}
		if err := clearDB(cliCtx.Context, dataDir, forceClearFlag); err != nil {
			return err
		}
	} else {
		dataFile := filepath.Join(dataDir, kv.ProtectionDbFileName)
		if !file.FileExists(dataFile) {
			log.Warnf("Slashing protection file %s is missing.\n"+
				"If you changed your --wallet-dir or --datadir, please copy your previous \"validator.db\" file into your current --datadir.\n"+
				"Disregard this warning if this is the first time you are running this set of keys.", dataFile)
		}
	}
	log.WithField("databasePath", dataDir).Info("Checking DB")

	valDB, err := kv.NewKVStore(cliCtx.Context, dataDir, &kv.Config{
		PubKeys:         nil,
		InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize db")
	}
	c.db = valDB
	if err := valDB.RunUpMigrations(cliCtx.Context); err != nil {
		return errors.Wrap(err, "could not run database migration")
	}

	if !cliCtx.Bool(cmd.DisableMonitoringFlag.Name) {
		if err := c.registerPrometheusService(cliCtx); err != nil {
			return err
		}
	}
	if err := c.registerValidatorService(cliCtx); err != nil {
		return err
	}
	if cliCtx.Bool(flags.EnableRPCFlag.Name) {
		if err := c.registerRPCService(cliCtx); err != nil {
			return err
		}
		if err := c.registerRPCGatewayService(cliCtx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ValidatorClient) initializeForWeb(cliCtx *cli.Context) error {
	var err error

	// Read the wallet password file from the cli context.
	if err = setWalletPasswordFilePath(cliCtx); err != nil {
		return errors.Wrap(err, "could not read wallet password file")
	}

	// Read the wallet from the specified path.
	w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
		return nil, nil
	})
	if err != nil {
		return errors.Wrap(err, "could not open wallet")
	}
	c.wallet = w
	dataDir := cliCtx.String(flags.WalletDirFlag.Name)
	if c.wallet != nil {
		dataDir = c.wallet.AccountsDir()
	}
	if cliCtx.String(cmd.DataDirFlag.Name) != cmd.DefaultDataDir() {
		dataDir = cliCtx.String(cmd.DataDirFlag.Name)
	}
	clearFlag := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearFlag := cliCtx.Bool(cmd.ForceClearDB.Name)

	if clearFlag || forceClearFlag {
		if dataDir == "" {
			dataDir = cmd.DefaultDataDir()
			if dataDir == "" {
				log.Fatal(
					"Could not determine your system'c HOME path, please specify a --datadir you wish " +
						"to use for your validator data",
				)
			}

		}
		if err := clearDB(cliCtx.Context, dataDir, forceClearFlag); err != nil {
			return err
		}
	}
	log.WithField("databasePath", dataDir).Info("Checking DB")
	valDB, err := kv.NewKVStore(cliCtx.Context, dataDir, &kv.Config{
		PubKeys:         nil,
		InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize db")
	}
	c.db = valDB
	if err := valDB.RunUpMigrations(cliCtx.Context); err != nil {
		return errors.Wrap(err, "could not run database migration")
	}

	if !cliCtx.Bool(cmd.DisableMonitoringFlag.Name) {
		if err := c.registerPrometheusService(cliCtx); err != nil {
			return err
		}
	}
	if err := c.registerValidatorService(cliCtx); err != nil {
		return err
	}
	if err := c.registerRPCService(cliCtx); err != nil {
		return err
	}
	if err := c.registerRPCGatewayService(cliCtx); err != nil {
		return err
	}
	gatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	gatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	webAddress := fmt.Sprintf("http://%s:%d", gatewayHost, gatewayPort)
	log.WithField("address", webAddress).Info(
		"Starting Prysm web UI on address, open in browser to access",
	)
	return nil
}

func (c *ValidatorClient) registerPrometheusService(cliCtx *cli.Context) error {
	var additionalHandlers []prometheus.Handler
	if cliCtx.IsSet(cmd.EnableBackupWebhookFlag.Name) {
		additionalHandlers = append(
			additionalHandlers,
			prometheus.Handler{
				Path:    "/db/backup",
				Handler: backup.BackupHandler(c.db, cliCtx.String(cmd.BackupWebhookOutputDir.Name)),
			},
		)
	}
	service := prometheus.NewService(
		fmt.Sprintf("%s:%d", c.cliCtx.String(cmd.MonitoringHostFlag.Name), c.cliCtx.Int(flags.MonitoringPortFlag.Name)),
		c.services,
		additionalHandlers...,
	)
	logrus.AddHook(prometheus.NewLogrusCollector())
	return c.services.RegisterService(service)
}

func (c *ValidatorClient) registerValidatorService(cliCtx *cli.Context) error {
	endpoint := c.cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	dataDir := c.cliCtx.String(cmd.DataDirFlag.Name)
	logValidatorBalances := !c.cliCtx.Bool(flags.DisablePenaltyRewardLogFlag.Name)
	emitAccountMetrics := !c.cliCtx.Bool(flags.DisableAccountMetricsFlag.Name)
	cert := c.cliCtx.String(flags.CertFlag.Name)
	graffiti := c.cliCtx.String(flags.GraffitiFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	var interopKeysConfig *imported.InteropKeymanagerConfig
	if c.cliCtx.IsSet(flags.InteropNumValidators.Name) {
		interopKeysConfig = &imported.InteropKeymanagerConfig{
			Offset:           cliCtx.Uint64(flags.InteropStartIndex.Name),
			NumValidatorKeys: cliCtx.Uint64(flags.InteropNumValidators.Name),
		}
	}

	gStruct := &g.Graffiti{}
	var err error
	if c.cliCtx.IsSet(flags.GraffitiFileFlag.Name) {
		n := c.cliCtx.String(flags.GraffitiFileFlag.Name)
		gStruct, err = g.ParseGraffitiFile(n)
		if err != nil {
			log.WithError(err).Warn("Could not parse graffiti file")
		}
	}

	wsc, err := web3SignerConfig(c.cliCtx)
	if err != nil {
		return err
	}

	v, err := client.NewValidatorService(c.cliCtx.Context, &client.Config{
		Endpoint:                   endpoint,
		DataDir:                    dataDir,
		LogValidatorBalances:       logValidatorBalances,
		EmitAccountMetrics:         emitAccountMetrics,
		CertFlag:                   cert,
		GraffitiFlag:               g.ParseHexGraffiti(graffiti),
		GrpcMaxCallRecvMsgSizeFlag: maxCallRecvMsgSize,
		GrpcRetriesFlag:            grpcRetries,
		GrpcRetryDelay:             grpcRetryDelay,
		GrpcHeadersFlag:            c.cliCtx.String(flags.GrpcHeadersFlag.Name),
		ValDB:                      c.db,
		UseWeb:                     c.cliCtx.Bool(flags.EnableWebFlag.Name),
		InteropKeysConfig:          interopKeysConfig,
		Wallet:                     c.wallet,
		WalletInitializedFeed:      c.walletInitialized,
		GraffitiStruct:             gStruct,
		LogDutyCountDown:           c.cliCtx.Bool(flags.EnableDutyCountDown.Name),
		Web3SignerConfig:           wsc,
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize validator service")
	}

	return c.services.RegisterService(v)
}

func web3SignerConfig(cliCtx *cli.Context) (*remote_web3signer.SetupConfig, error) {
	var web3signerConfig *remote_web3signer.SetupConfig
	if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) && cliCtx.IsSet(flags.Web3SignerPublicValidatorKeysFlag.Name) {
		urlStr := cliCtx.String(flags.Web3SignerURLFlag.Name)
		publicKeysStr := cliCtx.String(flags.Web3SignerPublicValidatorKeysFlag.Name)
		u, err := url.Parse(urlStr)
		if err != nil {
			return nil, errors.Wrapf(err, "web3signer url %s is invalid", urlStr)
		}
		web3signerConfig = &remote_web3signer.SetupConfig{
			BaseEndpoint:          u.String(),
			GenesisValidatorsRoot: nil,
		}
		u, err = url.Parse(publicKeysStr)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse %s as a URL for web3signer remote public keys", publicKeysStr)
		}
		if u != nil {
			web3signerConfig.PublicKeysURL = publicKeysStr
		} else {
			var validatorKeys [][48]byte
			for _, key := range strings.Split(publicKeysStr, ",") {
				decodedKey, err := hexutil.Decode(key)
				if err != nil {
					return nil, errors.Wrapf(err, "could not decode public key for web3signer: %s", key)
				}
				validatorKeys = append(validatorKeys, bytesutil.ToBytes48(decodedKey))
			}
			web3signerConfig.ProvidedPublicKeys = validatorKeys
		}
	}
	return web3signerConfig, nil
}

func (c *ValidatorClient) registerRPCService(cliCtx *cli.Context) error {
	var vs *client.ValidatorService
	if err := c.services.FetchService(&vs); err != nil {
		return err
	}
	validatorGatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	validatorGatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	validatorMonitoringHost := cliCtx.String(cmd.MonitoringHostFlag.Name)
	validatorMonitoringPort := cliCtx.Int(flags.MonitoringPortFlag.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	nodeGatewayEndpoint := cliCtx.String(flags.BeaconRPCGatewayProviderFlag.Name)
	beaconClientEndpoint := cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	grpcHeaders := c.cliCtx.String(flags.GrpcHeadersFlag.Name)
	clientCert := c.cliCtx.String(flags.CertFlag.Name)
	server := rpc.NewServer(cliCtx.Context, &rpc.Config{
		ValDB:                    c.db,
		Host:                     rpcHost,
		Port:                     fmt.Sprintf("%d", rpcPort),
		WalletInitializedFeed:    c.walletInitialized,
		ValidatorService:         vs,
		SyncChecker:              vs,
		GenesisFetcher:           vs,
		NodeGatewayEndpoint:      nodeGatewayEndpoint,
		WalletDir:                walletDir,
		Wallet:                   c.wallet,
		ValidatorGatewayHost:     validatorGatewayHost,
		ValidatorGatewayPort:     validatorGatewayPort,
		ValidatorMonitoringHost:  validatorMonitoringHost,
		ValidatorMonitoringPort:  validatorMonitoringPort,
		BeaconClientEndpoint:     beaconClientEndpoint,
		ClientMaxCallRecvMsgSize: maxCallRecvMsgSize,
		ClientGrpcRetries:        grpcRetries,
		ClientGrpcRetryDelay:     grpcRetryDelay,
		ClientGrpcHeaders:        strings.Split(grpcHeaders, ","),
		ClientWithCert:           clientCert,
	})
	return c.services.RegisterService(server)
}

func (c *ValidatorClient) registerRPCGatewayService(cliCtx *cli.Context) error {
	gatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	if gatewayHost != flags.DefaultGatewayHost {
		log.WithField("web-host", gatewayHost).Warn(
			"You are using a non-default web host. Web traffic is served by HTTP, so be wary of " +
				"changing this parameter if you are exposing this host to the Internet!",
		)
	}
	gatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	rpcAddr := fmt.Sprintf("%s:%d", rpcHost, rpcPort)
	gatewayAddress := fmt.Sprintf("%s:%d", gatewayHost, gatewayPort)
	allowedOrigins := strings.Split(cliCtx.String(flags.GPRCGatewayCorsDomain.Name), ",")
	maxCallSize := cliCtx.Uint64(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)

	registrations := []gateway.PbHandlerRegistration{
		validatorpb.RegisterAuthHandler,
		validatorpb.RegisterWalletHandler,
		pb.RegisterHealthHandler,
		validatorpb.RegisterHealthHandler,
		validatorpb.RegisterAccountsHandler,
		validatorpb.RegisterBeaconHandler,
		validatorpb.RegisterSlashingProtectionHandler,
		ethpbservice.RegisterKeyManagementHandler,
	}
	gwmux := gwruntime.NewServeMux(
		gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &gwruntime.HTTPBodyMarshaler{
			Marshaler: &gwruntime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitUnpopulated: true,
					UseProtoNames:   true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
		gwruntime.WithMarshalerOption(
			"text/event-stream", &gwruntime.EventSourceJSONPb{},
		),
	)
	muxHandler := func(apiMware *apimiddleware.ApiProxyMiddleware, h http.HandlerFunc, w http.ResponseWriter, req *http.Request) {
		// The validator gateway handler requires this special logic as it serves two kinds of APIs, namely
		// the standard validator keymanager API under the /eth namespace, and the Prysm internal
		// validator API under the /api namespace. Finally, it also serves requests to host the validator web UI.
		if strings.HasPrefix(req.URL.Path, "/api/eth/") {
			req.URL.Path = strings.Replace(req.URL.Path, "/api", "", 1)
			// If the prefix has /eth/, we handle it with the standard API gateway middleware.
			apiMware.ServeHTTP(w, req)
		} else if strings.HasPrefix(req.URL.Path, "/api") {
			req.URL.Path = strings.Replace(req.URL.Path, "/api", "", 1)
			// Else, we handle with the Prysm API gateway without a middleware.
			h(w, req)
		} else {
			// Finally, we handle with the web server.
			web.Handler(w, req)
		}
	}

	pbHandler := &gateway.PbMux{
		Registrations: registrations,
		Patterns:      []string{"/accounts/", "/v2/", "/internal/eth/v1/"},
		Mux:           gwmux,
	}
	opts := []gateway.Option{
		gateway.WithRemoteAddr(rpcAddr),
		gateway.WithGatewayAddr(gatewayAddress),
		gateway.WithMaxCallRecvMsgSize(maxCallSize),
		gateway.WithPbHandlers([]*gateway.PbMux{pbHandler}),
		gateway.WithAllowedOrigins(allowedOrigins),
		gateway.WithApiMiddleware(&validatorMiddleware.ValidatorEndpointFactory{}),
		gateway.WithMuxHandler(muxHandler),
	}
	gw, err := gateway.New(cliCtx.Context, opts...)
	if err != nil {
		return err
	}
	return c.services.RegisterService(gw)
}

func setWalletPasswordFilePath(cliCtx *cli.Context) error {
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	defaultWalletPasswordFilePath := filepath.Join(walletDir, wallet.DefaultWalletPasswordFile)
	if file.FileExists(defaultWalletPasswordFilePath) {
		// Ensure file has proper permissions.
		hasPerms, err := file.HasReadWritePermissions(defaultWalletPasswordFilePath)
		if err != nil {
			return err
		}
		if !hasPerms {
			return fmt.Errorf(
				"wallet password file %s does not have proper 0600 permissions",
				defaultWalletPasswordFilePath,
			)
		}

		// Set the filepath into the cli context.
		if err := cliCtx.Set(flags.WalletPasswordFileFlag.Name, defaultWalletPasswordFilePath); err != nil {
			return errors.Wrap(err, "could not set default wallet password file path")
		}
	}
	return nil
}

func clearDB(ctx context.Context, dataDir string, force bool) error {
	var err error
	clearDBConfirmed := force

	if !force {
		actionText := "This will delete your validator's historical actions database stored in your data directory. " +
			"This may lead to potential slashing - do you want to proceed? (Y/N)"
		deniedText := "The historical actions database will not be deleted. No changes have been made."
		clearDBConfirmed, err = cmd.ConfirmAction(actionText, deniedText)
		if err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	if clearDBConfirmed {
		valDB, err := kv.NewKVStore(ctx, dataDir, &kv.Config{})
		if err != nil {
			return errors.Wrapf(err, "Could not create DB in dir %s", dataDir)
		}
		if err := valDB.Close(); err != nil {
			return errors.Wrapf(err, "could not close DB in dir %s", dataDir)
		}

		log.Warning("Removing database")
		if err := valDB.ClearDB(); err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	return nil
}
