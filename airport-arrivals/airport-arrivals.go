package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"
)

const (
	HTML_URL = "http://www.prg.aero/cs/informace-o-letech/prilety-a-odlety/prilety/?hour=all&destination=0&carrier=0&act=main-%3Eparam-%3Eparam-%3Eparam-%3EsetFiltr"

)

type Flight struct {
	Time  time.Time
	City  string
	Codes []string
}

func (flight Flight) String() string {
	return fmt.Sprintf("Time: %02d:%02d City: %s Codes: %+v", flight.Time.Hour(), flight.Time.Minute(), flight.City, flight.Codes)
}

func main() {
	page, err := GetHTMLPage(HTML_URL)
	if err != nil {
		panic(err)
	}

	lastUpdateTime, err := LastUpdate(page)
	if err != nil {
		panic(err)
	}

	fmt.Printf("lastUpdate: %s\n", lastUpdateTime)
	for _, flight := range Flights(page) {
		fmt.Printf("%s\n", flight)
	}
}

func GetHTMLPage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func LastUpdate(b []byte) (time.Time, error) {
	lastUpdateRE, err := regexp.Compile(`Poslední aktualizace: ([0-9.: ]*)</div>`)
	if err != nil {
		return time.Now(), err
	}

	lastUpdate := lastUpdateRE.FindSubmatch(b)

	return time.Parse("1.2.2006 15:04", string(lastUpdate[1]))
}

func Flights(b []byte) []*Flight {
	flightsRE, err := regexp.Compile(`<tr>\s*<td><div> ([0-9]{2}:[0-9]{2})</div></td>\s*<td><div><div class="flightNumber">([A-Z]{2}\ [0-9]{4})</div><span class="flID">\d*</span></div></td>\s*<td><div>(.*?)</div></td>`)
	if err != nil {
		panic(err)
	}

	flights := []*Flight{}
	var lastAddedFlight *Flight

	flightsRes := flightsRE.FindAllSubmatch(b, -1)
	for _, flight := range flightsRes {
		t, city, code, err := ParseFlightSubmatch(flight)
		if err != nil {
			fmt.Printf("Parsing time failed! %s\n", err)
			break
		}

		flight := &Flight{Time: t, City: city, Codes: []string{code}}

		if lastAddedFlight != nil && lastAddedFlight.Time == t && lastAddedFlight.City == city {
			lastAddedFlight.Codes = append(lastAddedFlight.Codes, code)
		} else {
			lastAddedFlight = flight
			flights = append(flights, lastAddedFlight)
		}
	}

	return flights
}

func ParseFlightSubmatch(flightSubmatch [][]byte) (time.Time, string, string, error) {
	t, err := time.Parse("15:04", string(flightSubmatch[1]))
	if err != nil {
		return time.Now(), "", "", err
	}

	return t, string(flightSubmatch[3]), string(flightSubmatch[2]), nil
}
