package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
)

type AirzoneSystem struct {
	Data []struct {
		SystemID   int           `json:"systemID"`
		ZoneID     int           `json:"zoneID"`
		Name       string        `json:"name,omitempty"`
		On         int           `json:"on"`
		MaxTemp    float64       `json:"maxTemp"`
		MinTemp    float64       `json:"minTemp"`
		Setpoint   float64       `json:"setpoint"`
		RoomTemp   float64       `json:"roomTemp"`
		ColdStages int           `json:"coldStages"`
		ColdStage  int           `json:"coldStage"`
		HeatStages int           `json:"heatStages"`
		HeatStage  int           `json:"heatStage"`
		Humidity   int           `json:"humidity"`
		Speed      int           `json:"speed"`
		Units      int           `json:"units"`
		Errors     []interface{} `json:"errors"`
		Modes      []int         `json:"modes,omitempty"`
		Mode       int           `json:"mode,omitempty"`
	} `json:"data"`
}

var influxURL, influxDB, influxUSER, influxPASS string
var airzoneIP string
var cnfLogfile string
var cnfDaemon bool
var logfile *os.File

func doLog(format string, a ...interface{}) {
	t := time.Now()

	if cnfLogfile != "" && logfile == os.Stdout {
		fmt.Printf("INFO: Creating log file '%s'\n", cnfLogfile)
		tf, err := os.OpenFile(cnfLogfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("ERROR: Can't open log file '%s' for writing, logging to Stdout\n", cnfLogfile)
			fmt.Printf("ERROR: %s\n", err)
		} else {
			fmt.Printf("INFO: log file '%s' is ready for writing\n", cnfLogfile)
			logfile = tf
		}
	}

	// logfile default is os.Stdout
	fmt.Fprintf(logfile, "%s ", string(t.Format("20060102 150405")))
	fmt.Fprintf(logfile, format, a...)
}

func pushInflux(az AirzoneSystem) {
	doLog("pushInflux call\n")
	// Create a new HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     influxURL,
		Username: influxUSER,
		Password: influxPASS,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		doLog("ERROR: Can't create HTTP client to influxdb :%s\n", err)
	}
	defer c.Close()

	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  influxDB,
		Precision: "s",
	})
	if err != nil {
		doLog("ERROR: Can't create batchpoint : %s\n", err)
		return
	}

	doLog("Report contains %d zones\n", len(az.Data))
	for i := 0; i < len(az.Data); i++ {

		// Create a point and add to batch
		tags := map[string]string{"systemid": strconv.Itoa(az.Data[i].SystemID), "zoneid": strconv.Itoa(az.Data[i].ZoneID)}
		fields := map[string]interface{}{"roomtemp": az.Data[i].RoomTemp, "humidity": az.Data[i].Humidity, "setpoint": az.Data[i].Setpoint}

		pt, err := client.NewPoint("metrics", tags, fields, time.Now())

		if err != nil {
			doLog("ERROR: Can't create InfluxDB points: %s\n", err)
			return
		}
		doLog("InfluxDB points %v\n", pt)
		bp.AddPoint(pt)
	}
	// Write the batch
	if err := c.Write(bp); err != nil {
		doLog("ERROR: Can't send batch '%s'\n", err)
		return
	}
	doLog("Transmission OK\n")

}

func airzoneWorker() {

	url := "http://" + airzoneIP + ":3000/api/v1/hvac"

	spaceClient := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	pay, err := json.Marshal(map[string]int{
		"systemid": 1,
		"zoneid":   0,
	})
	if err != nil {
		log.Fatal(err)
	}

	for {

		var err error
		var airSystem AirzoneSystem
		var body []byte
		var resp *http.Response
		var req *http.Request

		doLog("Connecting Airzone interface\n")
		req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(pay))
		if err != nil {
			doLog("Error creating POST Request %s\n", err.Error())
			goto end
		}

		req.Header.Set("User-Agent", "AirZombie")
		resp, err = spaceClient.Do(req)
		if err != nil {
			doLog("Error sending POST Request %s\n", err.Error())
			goto end
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			doLog("Error reading POST Request %s\n", err.Error())
			goto end
		}
		doLog("Airzone Response" + string(body))

		airSystem = AirzoneSystem{}
		err = json.Unmarshal(body, &airSystem)
		if err != nil {
			doLog("Error unmarshalling POST Response %s\n", err.Error())
			goto end
		}
		if len(airSystem.Data) > 0 {
			doLog("# of zones found in answer: %d\n", len(airSystem.Data))
			pushInflux(airSystem)
		} else {
			doLog("Json answer Data structure is empty")
		}

end:
		doLog("Pause for 60 sec\n")
		time.Sleep(60 * time.Second)
	}
}

func parseArgs() {
	flag.StringVar(&influxURL, "influx_url", "", "InfluxDB serveur URL")
	flag.StringVar(&influxDB, "influx_db", "", "InfluxDB database")
	flag.StringVar(&influxUSER, "influx_user", "", "InfluxDB user")
	flag.StringVar(&influxPASS, "influx_pass", "", "InfluxDB password")
	flag.StringVar(&airzoneIP, "airzone_ip", "", "Airzone Local API IP")
	flag.StringVar(&cnfLogfile, "log", "", "Path to log file")
	flag.BoolVar(&cnfDaemon, "daemon", false, "Run in background")
	flag.Parse()

	doLog("Config:\n  - InfluxDB backend: Url:%s Db:%s\n", influxURL, influxDB)
	doLog("- Airzone IP: %s\n", airzoneIP)

}

func main() {

	logfile = os.Stdout
	parseArgs()
	doLog("Starting Airzombie Gateway \n")

	airzoneWorker()

}
