// Copyright © 2017 Delving B.V. <info@delving.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/json"
	"fmt"
	log "log"
	"net/http"
	"strconv"

	"github.com/delving/rapid-saas/config"
	"github.com/delving/rapid-saas/hub3/fragments"
	"github.com/delving/rapid-saas/hub3/index"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/golang/protobuf/proto"
	//elastic "github.cocm/olivere/elastic"
	elastic "gopkg.in/olivere/elastic.v5"
)

// SearchResource is a struct for the Search routes
type SearchResource struct{}

// Routes returns the chi.Router
func (rs SearchResource) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/v2", getScrollResult)
	r.Get("/v2/{id}", func(w http.ResponseWriter, r *http.Request) {
		getSearchRecord(w, r)
		return
	})

	r.Get("/v1", func(w http.ResponseWriter, r *http.Request) {
		render.PlainText(w, r, `{"status": "not enabled"}`)
		return
	})
	r.Get("/v1/{id}", func(w http.ResponseWriter, r *http.Request) {
		render.PlainText(w, r, `{"status": "not enabled"}`)
		return
	})

	return r
}

func getScrollResult(w http.ResponseWriter, r *http.Request) {

	searchRequest, err := fragments.NewSearchRequest(r.URL.Query())
	if err != nil {
		log.Println("Unable to create Search request")
		return
	}

	// Echo requests when requested
	echoRequest := r.URL.Query().Get("echo")
	if echoRequest != "" {
		echo, err := searchRequest.Echo(echoRequest, 0)
		if err != nil {
			log.Println("Unable to echo request")
			log.Println(err)
			return
		}
		if echo != nil {
			render.JSON(w, r, echo)
			return
		}
	}

	s, err := searchRequest.ElasticSearchService(index.ESClient())
	if err != nil {
		log.Println("Unable to create Search Service")
		return
	}

	log.Println(echoRequest)
	if echoRequest == "searchService" {
		render.JSON(w, r, s)
		return
	}

	res, err := s.Do(ctx)
	if err != nil {
		log.Println("Unable to get search result.")
		log.Println(err)
		return
	}
	if res == nil {
		log.Printf("expected response != nil; got: %v", res)
		return
	}

	log.Println(echoRequest)
	if echoRequest == "searchResponse" {
		render.JSON(w, r, res)
		return
	}

	records, err := decodeFragmentGraphs(res)
	if err != nil {
		log.Printf("Unable to decode records")
		return
	}

	pager, err := searchRequest.NextScrollID(res.TotalHits())
	if err != nil {
		log.Println("Unable to create Scroll Pager. ")
		return
	}

	// Add scrollID pager information to the header
	w.Header().Add("P_SCROLL_ID", pager.GetScrollID())
	w.Header().Add("P_CURSOR", strconv.FormatInt(int64(pager.GetCursor()), 10))
	w.Header().Add("P_TOTAL", strconv.FormatInt(int64(pager.GetTotal()), 10))
	w.Header().Add("P_ROWS", strconv.FormatInt(int64(pager.GetRows()), 10))

	result := &fragments.ScrollResultV3{}
	result.Pager = pager
	result.Items = records
	switch searchRequest.GetResponseFormatType() {
	case fragments.ResponseFormatType_PROTOBUF:
		output, err := proto.Marshal(result)
		if err != nil {
			log.Println("Unable to marshal result to protobuf format.")
			return
		}
		render.Data(w, r, output)
	default:
		render.JSON(w, r, result)
	}
	return
}

func getSearchRecord(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := index.ESClient().Get().
		Index(config.Config.ElasticSearch.IndexName).
		Id(id).
		Do(ctx)
	if err != nil {
		log.Println("Unable to get search result.")
		log.Println(err)
		return
	}
	if res == nil {
		log.Printf("expected response != nil; got: %v", res)
		return
	}
	if !res.Found {
		log.Printf("%s was not found", id)
		return
	}

	record, err := decodeFragmentGraph(res.Source)
	if err != nil {
		fmt.Printf("Unable to decode RDFRecord: %#v", res.Source)
		return
	}
	switch r.URL.Query().Get("format") {
	case "protobuf":
		output, err := proto.Marshal(record)
		if err != nil {
			log.Println("Unable to marshal result to protobuf format.")
			return
		}
		render.Data(w, r, output)
	default:
		render.JSON(w, r, record)
	}
	return
}

func decodeFragmentGraph(hit *json.RawMessage) (*fragments.FragmentGraph, error) {
	r := new(fragments.FragmentGraph)
	if err := json.Unmarshal(*hit, r); err != nil {
		return nil, err
	}
	return r, nil
}

// decodeFragmentGraphs takes a search result and deserializes the records
func decodeFragmentGraphs(res *elastic.SearchResult) ([]*fragments.FragmentGraph, error) {
	if res == nil || res.TotalHits() == 0 {
		return nil, nil
	}

	var records []*fragments.FragmentGraph
	for _, hit := range res.Hits.Hits {
		r, err := decodeFragmentGraph(hit.Source)
		if err != nil {
			return nil, err
		}
		// remove RDF
		r.RDF = nil
		records = append(records, r)
	}
	return records, nil
}
