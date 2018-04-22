package models

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	c "github.com/delving/rapid-saas/config"
	"github.com/gammazero/workerpool"
	r "github.com/kiivihal/rdf2go"
	ld "github.com/linkeddata/gojsonld"
	"github.com/parnurzeal/gorequest"
)

// PostHookJob  holds the info for building a crea
type PostHookJob struct {
	Graph   *r.Graph
	Spec    string
	Deleted bool
	Subject string
}

// PostHookJobFactory can be used to fire off PostHookJob jobs
type PostHookJobFactory struct {
	Spec string
	wp   *workerpool.WorkerPool
}

// NewPostHookJob creates a new PostHookJob and populates the rdf2go Graph
func NewPostHookJob(g *r.Graph, spec string, delete bool, subject string) *PostHookJob {
	return &PostHookJob{g, spec, delete, subject}
}

// Valid determines if the posthok is valid to apply.
func (ph PostHookJob) Valid() bool {
	return ProcessSpec(ph.Spec)
}

// ProcessSpec determines if a PostHookJob should be applied for a specific spec
func ProcessSpec(spec string) bool {
	for _, e := range c.Config.PostHook.ExcludeSpec {
		if e == spec {
			return false
		}
	}
	return true
}

// ApplyPostHookJob applies the PostHookJob to all the configured URLs
func ApplyPostHookJob(ph *PostHookJob) {
	//time.Sleep(100 * time.Millisecond)
	for _, u := range c.Config.PostHook.URLs {
		err := ph.Post(u)
		if err != nil {
			log.Println(err)
			log.Printf("Unable to send %s to %s", ph.Subject, u)
		}
	}
}

// Post sends json-ld to the specified endpointt
func (ph PostHookJob) Post(url string) error {
	request := gorequest.New()
	if ph.Deleted {
		log.Printf("Deleting via posthook: %s", ph.Subject)
		deleteURL := fmt.Sprintf("%s/delete", url)
		req := request.Delete(deleteURL).
			Query(fmt.Sprintf("id=%s", ph.Subject)).
			Retry(3, 5*time.Second, http.StatusBadRequest, http.StatusInternalServerError, http.StatusRequestTimeout)
		//log.Printf("%v", req)
		rsp, body, errs := req.End()
		if errs != nil || rsp.StatusCode != http.StatusNoContent {
			log.Printf("post-response: %#v -> %#v\n %#v", rsp, body, errs)
			log.Printf("Unable to delete: %#v", errs)
			return fmt.Errorf("Unable to save %s to endpoint %s", ph.Subject, url)
		}
		log.Printf("Deleted %s\n", ph.Subject)
		return nil
	}
	ph.cleanPostHookGraph()
	json, err := ph.String()

	if err != nil {
		return err
	}

	rsp, body, errs := request.Post(url).
		Set("Content-Type", "application/json-ld; charset=utf-8").
		Type("text").
		Send(json).
		Retry(3, 5*time.Second, http.StatusBadRequest, http.StatusInternalServerError, http.StatusRequestTimeout).
		End()
	//fmt.Printf("jsonld: %s\n", json)
	if errs != nil || rsp.StatusCode != http.StatusOK {
		log.Printf("post-response: %#v -> %#v\n %#v", rsp, body, errs)
		log.Printf("Unable to store: %#v", errs)
		log.Printf("JSON-LD: %s\n", json)
		return fmt.Errorf("Unable to save %s to endpoint %s", ph.Subject, url)
	}
	log.Printf("Stored %s\n", ph.Subject)
	return nil
}

var (
	ns = struct {
		rdf, rdfs, acl, cert, foaf, stat, dc, dcterms, nave, rdagr2, edm ld.NS
	}{
		rdf:     ld.NewNS("http://www.w3.org/1999/02/22-rdf-syntax-ns#"),
		rdfs:    ld.NewNS("http://www.w3.org/2000/01/rdf-schema#"),
		acl:     ld.NewNS("http://www.w3.org/ns/auth/acl#"),
		cert:    ld.NewNS("http://www.w3.org/ns/auth/cert#"),
		foaf:    ld.NewNS("http://xmlns.com/foaf/0.1/"),
		stat:    ld.NewNS("http://www.w3.org/ns/posix/stat#"),
		dc:      ld.NewNS("http://purl.org/dc/elements/1.1/"),
		dcterms: ld.NewNS("http://purl.org/dc/terms/"),
		nave:    ld.NewNS("http://schemas.delving.eu/nave/terms/"),
		rdagr2:  ld.NewNS("http://rdvocab.info/ElementsGr2/"),
		edm:     ld.NewNS("http://www.europeana.eu/schemas/edm/"),
	}
)

var dateFields = []ld.Term{
	ns.dcterms.Get("created"),
	ns.dcterms.Get("issued"),
	ns.nave.Get("creatorBirthYear"),
	ns.nave.Get("creatorDeathYear"),
	ns.nave.Get("date"),
	ns.dc.Get("date"),
	ns.nave.Get("dateOfBurial"),
	ns.nave.Get("dateOfDeath"),
	ns.nave.Get("productionEnd"),
	ns.nave.Get("productionStart"),
	ns.nave.Get("productionPeriod"),
	ns.rdagr2.Get("dateOfBirth"),
	ns.rdagr2.Get("dateOfDeath"),
}

func cleanDates(g *r.Graph, t *r.Triple) bool {
	for _, date := range dateFields {
		if t.Predicate.RawValue() == date.RawValue() {
			newTriple := r.NewTriple(
				t.Subject,
				r.NewResource(fmt.Sprintf("%sRaw", t.Predicate.RawValue())),
				t.Object,
			)
			g.Add(newTriple)
			return true
		}
	}
	return false
}

// cleanPostHookGraph applies post hook clean actions to the graph
func (ph PostHookJob) cleanPostHookGraph() {
	newGraph := r.NewGraph("")
	for t := range ph.Graph.IterTriples() {
		if !cleanDates(newGraph, t) {
			newGraph.Add(t)
		}
	}
	ph.Graph = newGraph
}

// Bytes returns the PostHookJob as an JSON-LD bytes.Buffer
func (ph PostHookJob) Bytes() (bytes.Buffer, error) {
	var b bytes.Buffer
	err := ph.Graph.SerializeFlatJSONLD(&b)
	if err != nil {
		return b, err
	}
	return b, nil
}

// Bytes returns the PostHookJob as an JSON-LD string
func (ph PostHookJob) String() (string, error) {
	b, err := ph.Bytes()
	if err != nil {
		return "", err
	}
	return b.String(), nil
}