package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	zlog "github.com/rs/zerolog/log"

	"github.com/go-k8s-metadata/diff"
	"github.com/go-k8s-metadata/tika"
	"github.com/k8-proxy/k8-go-comm/pkg/minio"
	"github.com/k8-proxy/k8-go-comm/pkg/rabbitmq"
	miniov7 "github.com/minio/minio-go/v7"
	"github.com/streadway/amqp"
)

// Flags requiring input.
const (
	parse    = "parse"
	detect   = "detect"
	language = "language"
	meta     = "meta"
)

// Informational flags which don't require input.
const (
	version   = "version"
	parsers   = "parsers"
	mimeTypes = "mimetypes"
	detectors = "detectors"
)

// Command line flags.
var (
	fileorgname                    string
	filerebname                    string
	metaField                      string
	recursive                      bool = false
	serverURL                           = os.Getenv("Tika_ENDPOINT")
	minioEndpoint                       = os.Getenv("MINIO_ENDPOINT")
	minioAccessKey                      = os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey                      = os.Getenv("MINIO_SECRET_KEY")
	adaptationRequestQueueHostname      = os.Getenv("ADAPTATION_REQUEST_QUEUE_HOSTNAME")
	adaptationRequestQueuePort          = os.Getenv("ADAPTATION_REQUEST_QUEUE_PORT")
	messagebrokeruser                   = os.Getenv("MESSAGE_BROKER_USER")
	messagebrokerpassword               = os.Getenv("MESSAGE_BROKER_PASSWORD")
	cleanMinioBucket                    = os.Getenv("MINIO_CLEAN_BUCKET")

	ProcessingRequestExchange   = "comparison-request-exchange"
	ProcessingRequestRoutingKey = "comparison-request"
	ProcessingRequestQueueName  = "comparison-request-queue"

	ProcessingOutcomeExchange   = "results-submission-exchange"
	ProcessingOutcomeRoutingKey = "results-submission"
	ProcessingOutcomeQueueName  = "results-submission-queue"
	fileorg                     io.Reader
	filereb                     io.Reader
	TikaClient                  *tika.Client
	minioClient                 *miniov7.Client
	publisher                   *amqp.Channel
)

func main() {
	var err error
	if serverURL == "" {
		serverURL = "http://localhost:9998"
	} else {
		fmt.Println(serverURL)
	}
	if serverURL == "" {
		log.Fatal("no URL specified: set serverURL, serverJAR and/or downloadVersion")
	}
	minioClient, err = minio.NewMinioClient(minioEndpoint, minioAccessKey, minioSecretKey, false)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start minio client ")
	}
	// Get a connection
	connection, err := rabbitmq.NewInstance(adaptationRequestQueueHostname, adaptationRequestQueuePort, messagebrokeruser, messagebrokerpassword)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq connection ")
	}
	// Initiate a publisher on processing exchange
	publisher, err = rabbitmq.NewQueuePublisher(connection, ProcessingOutcomeExchange, amqp.ExchangeDirect)
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq publisher ")
	}

	// Start a consumer
	msgs, ch, err := rabbitmq.NewQueueConsumer(connection, ProcessingRequestQueueName, ProcessingRequestExchange, amqp.ExchangeDirect, ProcessingRequestRoutingKey, amqp.Table{})
	if err != nil {
		zlog.Fatal().Err(err).Msg("error could not start rabbitmq consumer ")
	}
	defer ch.Close()

	TikaClient = tika.NewClient(nil, serverURL)

	forever := make(chan bool)

	// Consume
	go func() {
		for d := range msgs {
			fmt.Println("meta received message")

			zlog.Info().Msg("received message from queue ")

			err := processTikaMessage(d.Headers)
			if err != nil {
				zlog.Error().Err(err).Msg("error Failed to process message")
			}

			// Closing the channel to exit
			zlog.Info().Msg(" closing the channel")
		}
	}()

	log.Printf("Waiting for messages")
	<-forever

}
func processTikaMessage(d amqp.Table) error {

	fileID := d["file-id"].(string)
	sourcePresignedURL := d["source-presigned-url"].(string)
	fsource, err := getFile(sourcePresignedURL)
	if err != nil {
		return fmt.Errorf("error failed to download from Minio:%s", err)
	}

	rs := bytes.NewReader(fsource)
	strs, err := process("meta", rs)

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("tika error: %v", err)
	}

	minioUploadProcess([]byte(strs), fileID+"-source-mete", ".txt", "meta-presigned-url", d)
	rs = bytes.NewReader(fsource)

	strsource, err := process("parse", rs)

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("tika error: %v", err)
	}

	cleanPresignedURL := d["clean-presigned-url"].(string)

	fclean, err := getFile(cleanPresignedURL)
	if err != nil {
		return fmt.Errorf("error failed to download from Minio:%s", err)
	}

	rc := bytes.NewReader(fclean)
	strc, err := process("meta", rc)

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("tika error: %v", err)
	}

	minioUploadProcess([]byte(strc), fileID+"-clean-mete", ".txt", "clean-presigned-url", d)
	rc = bytes.NewReader(fclean)

	strclean, err := process("parse", rc)

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("tika error: %v", err)
	}
	// strsource  strclean
	dif := diff.CharacterDiff(strsource, strclean)

	minioUploadProcess([]byte(dif), fileID+"-diff", ".txt", "comparison-url", d)

	return nil

}

func process(action string, file io.Reader) (string, error) {
	if TikaClient == nil {
		return "", errors.New("no client")
	}

	switch action {
	default:
		flag.Usage()
		return "", fmt.Errorf("error: invalid action %q", action)
	case parse:
		if recursive {
			bs, err := TikaClient.ParseRecursive(context.Background(), file)
			if err != nil {
				return "", err
			}
			return strings.Join(bs, "\n"), nil
		}
		return TikaClient.Parse(context.Background(), file)
	case detect:
		return TikaClient.Detect(context.Background(), file)
	case language:
		return TikaClient.Language(context.Background(), file)
	case meta:
		if metaField != "" {
			return TikaClient.MetaField(context.Background(), file, metaField)
		}
		if recursive {
			mr, err := TikaClient.MetaRecursive(context.Background(), file)
			if err != nil {
				return "", err
			}
			bytes, err := json.MarshalIndent(mr, "", "  ")
			if err != nil {
				return "", err
			}
			return string(bytes), nil
		}
		return TikaClient.Meta(context.Background(), file)
	case version:
		return TikaClient.Version(context.Background())
	case parsers:
		p, err := TikaClient.Parsers(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	case mimeTypes:
		mt, err := TikaClient.MIMETypes(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(mt, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	case detectors:
		d, err := TikaClient.Detectors(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}
func getFile(url string) ([]byte, error) {

	if minioClient == nil {
		return nil, fmt.Errorf("minio client not found")
	}

	f := []byte{}
	resp, err := http.Get(url)
	if err != nil {

		return f, err
	}
	defer resp.Body.Close()

	f, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return f, err
	}
	return f, nil

}

func uploadMinio(file []byte, filename string) (string, error) {
	if minioClient == nil {
		return "", fmt.Errorf("minio client not found")
	}

	exist, err := minio.CheckIfBucketExists(minioClient, cleanMinioBucket)
	if err != nil || !exist {
		return "", err

	}
	_, errm := minio.UploadFileToMinio(minioClient, cleanMinioBucket, filename, bytes.NewReader(file))
	if errm != nil {
		return "", errm
	}

	expirein := time.Second * 60 * 2
	urlx, err := minio.GetPresignedURLForObject(minioClient, cleanMinioBucket, filename, expirein)
	if err != nil {
		return "", err

	}

	return urlx.String(), nil

}

func minioUploadProcess(file []byte, baseName, extName, headername string, d amqp.Table) {
	reportid := fmt.Sprintf("%s%s", baseName, extName)

	urlr, err := uploadMinio(file, reportid)
	if err != nil {
		m := fmt.Sprintf("failed to upload %s file to Minio", extName)
		zlog.Info().Msg(m)
		return
	}
	m := fmt.Sprintf("%s file uploaded to minio successfully", extName)

	zlog.Info().Msg(m)
	d[headername] = urlr
}
