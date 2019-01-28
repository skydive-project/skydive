package main

import (
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber/flowtransformer"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/websocket"
)

const defaultConfigurationFile = "/etc/skydive/skydive-objectstore.yml"

func main() {
	if err := config.InitConfig("file", []string{defaultConfigurationFile}); err != nil {
		logging.GetLogger().Errorf("Failed to initialize config: %s", err.Error())
		os.Exit(1)
	}

	if err := config.InitLogging(); err != nil {
		logging.GetLogger().Errorf("Failed to initialize logging system: %s", err.Error())
		os.Exit(1)
	}

	cfg := config.GetConfig()

	endpoint := cfg.GetString("endpoint")
	region := cfg.GetString("region")
	bucket := cfg.GetString("bucket")
	accessKey := cfg.GetString("access_key")
	secretKey := cfg.GetString("secret_key")
	objectPrefix := cfg.GetString("object_prefix")
	subscriberURLString := cfg.GetString("subscriber_url")
	subscriberUsername := cfg.GetString("subscriber_username")
	subscriberPassword := cfg.GetString("subscriber_password")
	maxSecondsPerStream := cfg.GetInt("max_seconds_per_stream")
	flowTransformerName := cfg.GetString("flow_transformer")

	flowTransformer, err := flowtransformer.New(flowTransformerName)
	if err != nil {
		logging.GetLogger().Errorf("Failed to initialize flow transformer: %s", err.Error())
		os.Exit(1)
	}

	authOpts := &shttp.AuthenticationOpts{
		Username: subscriberUsername,
		Password: subscriberPassword,
	}

	subscriberURL, err := url.Parse(subscriberURLString)
	if err != nil {
		logging.GetLogger().Errorf("Failed to parse subscriber URL: %s", err.Error())
		os.Exit(1)
	}

	wsClient, err := config.NewWSClient(common.AnalyzerService, subscriberURL, websocket.ClientOpts{AuthOpts: authOpts})
	if err != nil {
		logging.GetLogger().Errorf("Failed to create websocket client: %s", err)
		os.Exit(1)
	}
	structClient := wsClient.UpgradeToStructSpeaker()

	s := subscriber.New(endpoint, region, bucket, accessKey, secretKey, objectPrefix, maxSecondsPerStream, flowTransformer)

	// subscribe to the flow updates
	structClient.AddStructMessageHandler(s, []string{"flow"})
	structClient.Start()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	structClient.Stop()
}
