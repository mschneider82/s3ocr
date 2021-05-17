package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/mschneider82/s3ocr/seafile"
)

var (
	endpoint  = kingpin.Flag("endpoint", "s3 endpoint").Required().String()
	accesskey = kingpin.Flag("accesskey", "s3 accesskey").Required().String()
	secretkey = kingpin.Flag("secret", "s3 secret").OverrideDefaultFromEnvar("S3_SECRET").Required().String()
	useSSL    = kingpin.Flag("useSSL", "s3 ssl").Bool()
	bucket    = kingpin.Flag("bucket", "s3 bucket").Required().String()
	object    = kingpin.Flag("object", "s3 object/filename").Required().String()

	seafileserver    = kingpin.Flag("seafileserver", "url to seafile server").Required().String()
	seafiletoken     = kingpin.Flag("seafiletoken", "token see https://download.seafile.com/published/web-api/home.md").OverrideDefaultFromEnvar("SEAFILE_TOKEN").Required().String()
	seafilelibraryid = kingpin.Flag("seafilelibraryid", "e.g. 3e040126-4533-4d0c-97f3-baa284915515").Required().String()
)

func insecureTransport() http.RoundTripper {
	// Keep TLS config.
	tlsConfig := &tls.Config{}
	tlsConfig.InsecureSkipVerify = true

	var transport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
		// Set this value so that the underlying transport round-tripper
		// doesn't try to auto decode the body of objects with
		// content-encoding set to `gzip`.
		//
		// Refer:
		//    https://golang.org/src/net/http/transport.go?h=roundTrip#L1843
		DisableCompression: true,
	}
	return transport
}

func main() {
	kingpin.Parse()

	inputfilename := "todo-" + *object
	outputfilename := *object
	// Initialize minio client object.
	minioClient, err := minio.New(*endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(*accesskey, *secretkey, ""),
		Secure:    *useSSL,
		Transport: insecureTransport(),
	})
	if err != nil {
		log.Fatalln(err)
	}
	ctx := context.TODO()
	o, err := minioClient.GetObject(ctx, *bucket, *object, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.OpenFile(inputfilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = io.Copy(f, o)
	if err != nil {
		log.Fatalln(err)
	}

	err = f.Close()
	if err != nil {
		log.Fatalln(err)
	}

	err = o.Close()
	if err != nil {
		log.Fatalln(err)
	}

	cmd := exec.Command("ocrmypdf",
		"--deskew", "--tesseract-timeout",
		"2400", "--skip-big", "50", "-l", "deu",
		inputfilename, outputfilename)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("%s\n", out.String())

	of, err := os.Open(outputfilename)
	if err != nil {
		log.Fatalln(err)
	}
	defer of.Close()

	seafileprov := seafile.NewProvider(*seafileserver, *seafiletoken, *seafilelibraryid)
	fid, err := seafileprov.Upload(of, "/")
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("finished: %s \n", fid)

}
