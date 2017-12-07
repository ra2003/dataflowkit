package parse

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/slotix/dataflowkit/errs"
	"github.com/slotix/dataflowkit/fetch"
	"github.com/slotix/dataflowkit/scrape"
	"github.com/slotix/dataflowkit/splash"
	"github.com/spf13/viper"
)

// Define service interface
type Service interface {
	Parse(scrape.Payload) (io.ReadCloser, error)
}

// Implement service with empty struct
type ParseService struct {
}

type ServiceMiddleware func(Service) Service

//Parse calls Fetcher which downloads web page content for parsing
func (ps ParseService) Parse(p scrape.Payload) (io.ReadCloser, error) {
	config, err := p.PayloadToScrapeConfig()
	if err != nil {
		return nil, err
	}

	url := p.Request.(fetch.FetchRequester).GetURL()
	req := splash.Request{URL: url}
	//req := scrape.HttpClientFetcherRequest{URL: ps.GetURL(p.Request)}

	scraper, err := scrape.New(config)
	if err != nil {
		return nil, err
	}

	results, err := ps.scrape(req, scraper)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch config.Opts.Format {
	case "json":
		if config.Opts.PaginateResults {
			json.NewEncoder(&buf).Encode(results)
		} else {
			json.NewEncoder(&buf).Encode(results.AllBlocks())
		}
	case "csv":
		/*
			includeHeader := true
			w := csv.NewWriter(&buf)
			for i, page := range results.Results {
				if i != 0 {
					includeHeader = false
				}
				err = encodeCSV(names, includeHeader, page, ",", w)
				if err != nil {
					logger.Println(err)
				}
			}
			w.Flush()
		*/
		w := csv.NewWriter(&buf)

		err = encodeCSV(config.CSVHeader, results.AllBlocks(), ",", w)
		w.Flush()
	/*
		case "xmlviajson":
			var jbuf bytes.Buffer
			if config.Opts.PaginateResults {
				json.NewEncoder(&jbuf).Encode(results)
			} else {
				json.NewEncoder(&jbuf).Encode(results.AllBlocks())
			}
			//var buf bytes.Buffer
			m, err := mxj.NewMapJson(jbuf.Bytes())
			err = m.XmlIndentWriter(&buf, "", "  ")
			if err != nil {
				logger.Println(err)
			}
	*/
	case "xml":
		err = encodeXML(results.AllBlocks(), &buf)
		if err != nil {
			return nil, err
		}
	}
	readCloser := ioutil.NopCloser(bytes.NewReader(buf.Bytes()))
	return readCloser, nil
}

//responseFromFetchService sends request to fetch service and returns *splash.Response
func responseFromFetchService(req splash.Request) (*splash.Response, error) {

	//fetch content
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(b)
	addr := "http://" + viper.GetString("DFK_FETCH") + "/response/splash"
	request, err := http.NewRequest("POST", addr, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	r, err := client.Do(request)
	if r != nil {
		defer r.Body.Close()
	}
	if err != nil {
		panic(err)
	}
	resp, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var sResponse *splash.Response
	if err := json.Unmarshal(resp, &sResponse); err != nil {
		logger.Println("Json Unmarshall error", err)
	}
	//	content, err := sResponse.GetContent()
	//	if err != nil {
	//		return nil, err
	//	}
	//return content, nil
	return sResponse, nil
}

func (ps ParseService) scrape(req interface{}, scraper *scrape.Scraper) (*scrape.ScrapeResults, error) {
	url := req.(fetch.FetchRequester).GetURL()
	//get Robotstxt Data
	robotsData, err := fetch.RobotstxtData(url)
	if err != nil {
		return nil, err
	}
	res := &scrape.ScrapeResults{
		URLs:    []string{},
		Results: [][]map[string]interface{}{},
	}
	var numPages int
	//var retryTimes int
	for {
		//check if scraping of current url is not forbidden
		if !fetch.AllowedByRobots(url, robotsData) {
			return nil, &errs.ForbiddenByRobots{url}
		}
		// Repeat until we don't have any more URLs, or until we hit our page limit.
		if len(url) == 0 || (scraper.Config.Opts.MaxPages > 0 && numPages >= scraper.Config.Opts.MaxPages) {
			break
		}
		//call remote fetcher to download web page 
		sResponse, err := responseFromFetchService(req.(splash.Request))
		//sResponse, err := req.(splash.Request).GetResponse()
		
		if err != nil {
			return nil, err
		}
		content, err := sResponse.GetContent()
		if err != nil {
			return nil, err
		}
		// Create a goquery document.
		doc, err := goquery.NewDocumentFromReader(content)

		if err != nil {
			return nil, err
		}

		res.URLs = append(res.URLs, url)
		results := []map[string]interface{}{}

		// Divide this page into blocks
		for _, block := range scraper.Config.DividePage(doc.Selection) {
			blockResults := map[string]interface{}{}

			// Process each piece of this block
			for _, piece := range scraper.Config.Pieces {
				sel := block
				if piece.Selector != "." {
					sel = sel.Find(piece.Selector)
				}

				pieceResults, err := piece.Extractor.Extract(sel)
				if err != nil {
					return nil, err
				}

				// A nil response from an extractor means that we don't even include it in
				// the results.
				if pieceResults == nil {
					continue
				}

				blockResults[piece.Name] = pieceResults
			}
			if len(blockResults) > 0 {
				// Append the results from this block.
				results = append(results, blockResults)
			}
		}

		// Append the results from this page.
		res.Results = append(res.Results, results)

		numPages++

		// Get the next page.
		url, err = scraper.Config.Paginator.NextPage(url, doc.Selection)
		if err != nil {
			return nil, err
		}

		//every time when getting a response the next request will be filled with updated cookie information
		sRequest := req.(splash.Request)
		//	if response, ok := sResponse.(*splash.Response); ok {
		//sRequest := req.(splash.Request)
		err = sResponse.SetCookieToNextRequest(&sRequest)
		if err != nil {
			//return nil, err
			logger.Println(err)
		}
		sRequest.URL = url
		req = sRequest
		//req.URL = url

		if scraper.Config.Opts.RandomizeFetchDelay {
			//Sleep for time equal to FetchDelay * random value between 500 and 1500 msec
			rand := scrape.Random(500, 1500)
			delay := scraper.Config.Opts.FetchDelay * time.Duration(rand) / 1000
			logger.Println(delay)
			time.Sleep(delay)
		} else {
			time.Sleep(scraper.Config.Opts.FetchDelay)
		}

	}

	// All good!
	return res, nil

}