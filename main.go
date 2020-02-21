package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gocarina/gocsv"
)

type (
	latLng struct {
		Lat string `csv:"latitude"`
		Lng string `csv:"longitude"`
	}

	Matching struct {
		Confidence float64
	}

	mapBoxRes struct {
		Code string `json:"code"`
		// Tracepoints
		Matchings []Matching `json:"matchings"`
	}
)

const (
	mapBoxEndpoint    = "https://api.mapbox.com/matching/v5/mapbox/driving?access_token=%s"
	bodyCoordinates   = "coordinates=%s"
	mapBoxContentType = "application/x-www-form-urlencoded"
)

var (
	mapBoxAPIKey = os.Getenv("MAPBOX_API_KEY")
)

// Just gonna panic everywhere for now

func main() {
	var pointsFile, err = os.OpenFile("sampletrip.csv", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		panic(err)
	}

	defer pointsFile.Close()

	var points []*latLng

	err = gocsv.UnmarshalFile(pointsFile, &points)
	if err != nil { // Load clients from file
		panic(err)
	}

	var (
		coords        []string
		confidenceSum float64
		amount        int64
		wg            sync.WaitGroup
	)

	for i := range points {
		coords = append(coords, points[i].Lng+","+points[i].Lat)

		if i%50 == 0 && i > 49 {
			wg.Add(1)

			go func(start, end int, coords string) {
				defer wg.Done()

				var res, err = http.Post(fmt.Sprintf(mapBoxEndpoint, mapBoxAPIKey), mapBoxContentType, bytes.NewBufferString(fmt.Sprintf(bodyCoordinates, coords)))
				if err != nil {
					panic(err)
				}

				defer res.Body.Close()

				if res.StatusCode != http.StatusOK {
					panic(fmt.Errorf("code not 200: %d", res.StatusCode))
				}

				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					panic(err)
				}

				var mbRes mapBoxRes

				err = json.Unmarshal(body, &mbRes)
				if err != nil {
					panic(err)
				}

				if mbRes.Code != "Ok" {
					panic("Code is not 'Ok'")
				}

				if len(mbRes.Matchings) < 1 {
					panic("len(mbRes.Matchings) < 1")
				}

				fmt.Printf("Confidence for point [%3d, %3d]: %f\n", start, end, mbRes.Matchings[0].Confidence)

				confidenceSum += mbRes.Matchings[0].Confidence
				atomic.AddInt64(&amount, 1)
			}(i-50, i, strings.Join(coords, ";"))

			coords = []string{}
		}
	}

	wg.Wait()

	fmt.Printf("\nAverage Confidence: %f\n", confidenceSum/float64(amount))
}
