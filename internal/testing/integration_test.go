package testing

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/libdns/libdns"
	"github.com/libdns/namecheap"
)

var (
	apiKey      = flag.String("api-key", "", "Namecheap API key.")
	apiUser     = flag.String("username", "", "Namecheap API username.")
	apiEndpoint = flag.String("endpoint", "https://api.sandbox.namecheap.com/xml.response", "Namecheap API endpoint.")
	domain      = flag.String("domain", "", "Domain to test with of the form sld.tld <testing.com>")
	clientIP    = flag.String("client-ip", "", "Public IP address of client machine")
)

func TestIntegration(t *testing.T) {
	p := &namecheap.Provider{
		APIKey:      *apiKey,
		User:        *apiUser,
		APIEndpoint: *apiEndpoint,
		ClientIP:    *clientIP,
	}

	newRecords := []libdns.Record{
		libdns.RR{
			Type: "A",
			Name: "@",
			Data: "127.0.0.1",
			TTL:  time.Second * 1799,
		},
		libdns.RR{
			Type: "A",
			Name: "www",
			Data: "127.0.0.1",
			TTL:  time.Second * 1799,
		},
	}

	t.Logf("Appending: %d Records", len(newRecords))

	addedRecords, err := p.AppendRecords(context.TODO(), *domain, newRecords)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records appended: %#v", addedRecords)

	records, err := p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	// IDs are not returned by append. Maybe they should be?
	//ignoreIDField := cmpopts.IgnoreFields(libdns.RR{}, "ID")
	//if diff := cmp.Diff(addedRecords, records, ignoreIDField); diff != "" {
	if diff := cmp.Diff(addedRecords, records); diff != "" {
		t.Log(addedRecords, records)
		t.Fatalf("Added records not equal to fetched records. Diff: %s", diff)
	}

	firstRecord := records[0]

	t.Logf("Removing record: %#v", firstRecord)
	recordsRemoved, err := p.DeleteRecords(context.TODO(), *domain, []libdns.Record{firstRecord})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records removed: %#v", recordsRemoved)

	records, err = p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final number of records: %d", len(records))

	if len(records) != 1 {
		t.Fatalf("Expected 1 but got: %v", len(records))
	}
}

func TestSetRecordsKeepsExisting(t *testing.T) {
	p := &namecheap.Provider{
		APIKey:      *apiKey,
		User:        *apiUser,
		APIEndpoint: *apiEndpoint,
	}

	newRecords := []libdns.Record{
		libdns.RR{
			Type: "A",
			Name: "@",
			Data: "127.0.0.1",
			TTL:  time.Second * 1799,
		},
		libdns.RR{
			Type: "A",
			Name: "www",
			Data: "127.0.0.1",
			TTL:  time.Second * 1799,
		},
	}

	t.Logf("Appending: %d Records", len(newRecords))

	addedRecords, err := p.AppendRecords(context.TODO(), *domain, newRecords)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records appended: %#v", addedRecords)

	records, err := p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(records))
	}

	rr := records[0].RR()
	rr.Data = "0.0.0.0"
	records[0] = libdns.Record(rr)

	t.Log("Updating record")
	updatedRecords, err := p.SetRecords(context.TODO(), *domain, records)
	if err != nil {
		t.Fatal(err)
	}

	if len(updatedRecords) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(updatedRecords))
	}

	updatedRecords, err = p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Updated records: %#v", updatedRecords)

	if len(updatedRecords) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(updatedRecords))
	}

	var updatedRecord *libdns.Record
	for _, record := range updatedRecords {

		if record.RR().Data == "0.0.0.0" {
			updatedRecord = &record
		}
	}

	if updatedRecord == nil {
		t.Fatal("Record was never updated.")
	}
}
