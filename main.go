package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/influxdata/influxdb1-client" // needed for bug in go mod
	client "github.com/influxdata/influxdb1-client/v2"

	"github.com/joho/godotenv"
	"github.com/mcneilcode/go-enviro"
	"github.com/mcneilcode/go-schedule"
)

// PiholeConfig is a structure containing host and access related
// information for Pihole.
type PiholeConfig struct {
	Host      string
	Port      string
	APIRoute  string
	URLScheme string
	URL       string
}

// InfluxDBConfig is a structure containing host and access related
// information for InfluxDB.
type InfluxDBConfig struct {
	Host            string
	Port            string
	Username        string
	Password        string
	DbName          string
	Measurement     string
	RetentionPolicy string
	URLScheme       string
	URL             string
}

// Config contains base configuration for Sensor Monitor.
type Config struct {
	InfluxDB       InfluxDBConfig
	Pihole         PiholeConfig
	Hostname       string
	MetricDelay    int
	RequestTimeout int
	Request        *http.Client
}

// newConfig returns a new Config struct containing all current
// application settings.
func newConfig() *Config {

	var err error
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env configuration file. Error: %v", err)
	}

	// ignore expired SSL certificates
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	systemHostname, _ := os.Hostname()
	hostname := enviro.Get("METRIC_HOSTNAME", systemHostname)

	influxScheme := enviro.Get("INFLUXDB_URL_SCHEME", "https")
	influxHost := enviro.Get("INFLUXDB_HOST", "localhost")
	influxPort := enviro.Get("INFLUXDB_PORT", "8086")

	piholeScheme := enviro.Get("PIHOLE_URL_SCHEME", "https")
	piholeHost := enviro.Get("PIHOLE_HOST", "localhost")
	piholePort := enviro.Get("PIHOLE_PORT", "8080")

	return &Config{

		InfluxDB: InfluxDBConfig{
			Host:            influxHost,
			Port:            influxPort,
			Username:        enviro.Get("INFLUXDB_USERNAME", "telegraf"),
			Password:        enviro.Get("INFLUXDB_PASSWORD", "telegraf"),
			DbName:          enviro.Get("INFLUXDB_DATABASE", "telegraf"),
			Measurement:     enviro.Get("INFLUXDB_MEASUREMENT", "pihole"),
			RetentionPolicy: enviro.Get("INFLUXDB_RETENTION_POLICY", ""),
			URLScheme:       influxScheme,
			URL:             fmt.Sprintf("%s://%s:%s", influxScheme, influxHost, influxPort),
		},

		Pihole: PiholeConfig{
			Host:      piholeHost,
			Port:      piholePort,
			APIRoute:  enviro.Get("PIHOLE_API_ROUTE", ""),
			URLScheme: piholeScheme,
			URL:       piholeScheme + "://" + piholeHost + ":" + piholePort,
		},

		Hostname:       hostname,
		MetricDelay:    enviro.GetInt("METRIC_DELAY", 10),
		RequestTimeout: enviro.GetInt("REQUEST_TIMEOUT", 15),
		Request:        &http.Client{Transport: transCfg},
	}
}

// getStats gets the latest information from the pihole api
// and returns them in a byte array.
func (c Config) getStats() []byte {

	resp, err := c.Request.Get(c.Pihole.URL + c.Pihole.APIRoute)
	if err != nil {
		log.Println("Failed to query the Pihole admin api route at", c.Pihole.URL+c.Pihole.APIRoute)
		return []byte{}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}
	}
	return body
}

// influxWrite takes the payload from the pihole api as a byte array, converts it to json,
// and sends the measurement to InfluxDB using the influxdata go client.
func (c Config) influxWrite(payload []byte) error {

	config := client.HTTPConfig{
		Addr:     c.InfluxDB.URL,
		Username: c.InfluxDB.Username,
		Password: c.InfluxDB.Password,
		Timeout:  c.Request.Timeout,
	}
	con, _ := client.NewHTTPClient(config)
	defer con.Close()

	// Ensure connectivity to InfluxDB
	_, _, err := con.Ping(c.Request.Timeout)
	if err != nil {
		fmt.Println("No connectivity to InfluxDB, check configuration: ", err.Error())
		return err
	}

	// Convert the byte array to a map:
	var data map[string]interface{}
	json.NewDecoder(bytes.NewReader(payload)).Decode(&data)

	bp, err := client.NewBatchPoints(
		client.BatchPointsConfig{
			Database:  c.InfluxDB.DbName,
			Precision: "s",
		})
	if err != nil {
		log.Println("Error creating batch points:", err)
		return err
	}

	// Craft and send measurement:
	tags := map[string]string{"host": c.Hostname}
	fields := map[string]interface{}{
		"ads_percentage_today":  data["ads_percentage_today"].(float64),
		"ads_blocked_today":     data["ads_blocked_today"].(float64),
		"clients_ever_seen":     data["clients_ever_seen"].(float64),
		"domains_blocked":       data["domains_being_blocked"].(float64),
		"dns_queries_today":     data["dns_queries_today"].(float64),
		"dns_queries_all_types": data["dns_queries_all_types"].(float64),
		"queries_forwarded":     data["queries_forwarded"].(float64),
		"queries_cached":        data["queries_cached"].(float64),
		"reply_CNAME":           data["reply_CNAME"].(float64),
		"reply_IP":              data["reply_IP"].(float64),
		"reply_NODATA":          data["reply_NODATA"].(float64),
		"reply_NXDOMAIN":        data["reply_NXDOMAIN"].(float64),
		"unique_clients":        data["unique_clients"].(float64),
		"unique_domains":        data["unique_domains"].(float64),
	}

	pt, err := client.NewPoint(c.InfluxDB.Measurement, tags, fields, time.Now().UTC())
	if err != nil {
		fmt.Println("Error creating measurement:", err.Error())
		return err
	}

	bp.AddPoint(pt)
	err = con.Write(bp)
	if err != nil {
		log.Println("Error writing data to InfluxDB:", err.Error())
		return err
	}
	return nil
}

// collectStats fetches statistics information from the Pihole API and
// sends them into InfluxDB. It is meant to be called by the worker process.
func collectStats(ctx context.Context) {
	pihole := newConfig()
	stats := pihole.getStats()
	if len(stats) != 0 {
		pihole.influxWrite(stats)
	}
}

func main() {
	ctx := context.Background()
	conf := newConfig()

	worker := schedule.New()
	worker.Add(ctx, collectStats, time.Second*time.Duration(conf.MetricDelay))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Interrupt)
	<-quit
	worker.Stop()
}
